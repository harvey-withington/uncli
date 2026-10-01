// Package session owns session lifecycles: one CLI process per session at
// a time, a reader goroutine feeding the adapter's parser, the activity
// state machine, and page assembly into the store.
package session

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"uncli/internal/core"
	"uncli/internal/profile"
	"uncli/internal/store"
)

// BinaryFunc resolves the CLI binary to run and its version.
type BinaryFunc func(ctx context.Context) (path, version string, err error)

type Deps struct {
	Store      *store.Store
	Adapter    core.Adapter
	Runtime    core.Runtime
	Profiles   *profile.Set
	Binary     BinaryFunc
	Sink       Sink
	ScratchDir string // chat sessions get <ScratchDir>/<session id>
}

// ErrNoCLI means the CLI isn't installed yet; the UI shows the setup screen.
var ErrNoCLI = errors.New("the Claude CLI is not installed yet")

type Manager struct {
	d Deps

	mu       sync.Mutex
	sessions map[string]*Session
	focused  string
	models   []core.ModelInfo
	usage    *core.UsageLimit

	// InterruptGrace is how long an interrupt waits for the CLI before the
	// process is killed instead.
	InterruptGrace time.Duration
}

const modelsSetting = "claude.models"

func NewManager(d Deps) (*Manager, error) {
	m := &Manager{d: d, sessions: map[string]*Session{}, InterruptGrace: 5 * time.Second}
	if err := d.Store.MarkOpenPages("interrupted"); err != nil {
		return nil, err
	}
	recs, err := d.Store.ListSessions()
	if err != nil {
		return nil, err
	}
	for _, r := range recs {
		m.sessions[r.ID] = newSession(m, r)
	}
	if raw, _ := d.Store.Setting(modelsSetting); raw != "" {
		_ = json.Unmarshal([]byte(raw), &m.models)
	}
	return m, nil
}

func (m *Manager) get(id string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("no session %s", id)
	}
	return s, nil
}

// List returns every session, newest first.
func (m *Manager) List() []View {
	m.mu.Lock()
	all := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		all = append(all, s)
	}
	m.mu.Unlock()
	out := make([]View, 0, len(all))
	for _, s := range all {
		out = append(out, s.View())
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SortOrder != out[j].SortOrder {
			return out[i].SortOrder > out[j].SortOrder
		}
		return out[i].CreatedAt > out[j].CreatedAt
	})
	return out
}

func NewID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Create makes a new session. The process starts with the first turn.
func (m *Manager) Create(profileID, workdir, model string) (View, error) {
	p, ok := m.d.Profiles.Profile(profileID)
	if !ok {
		return View{}, fmt.Errorf("unknown profile %q", profileID)
	}
	if model == "" {
		model = p.Model
	}
	id := NewID()
	if p.Folder == "scratch" || workdir == "" {
		if p.Folder != "scratch" {
			return View{}, errors.New("choose a folder for this session")
		}
		workdir = filepath.Join(m.d.ScratchDir, id)
	}
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		return View{}, err
	}
	if p.Folder == "scratch" {
		_ = os.MkdirAll(filepath.Join(workdir, "artifacts"), 0o755)
	}
	rec := store.Session{ID: id, Adapter: m.d.Adapter.ID(), Runtime: m.d.Runtime.ID(), ProfileID: p.ID,
		Workdir: workdir, Model: model, Modifiers: append([]string{}, p.ModifiersOn...)}
	if err := m.d.Store.CreateSession(&rec); err != nil {
		return View{}, err
	}
	s := newSession(m, rec)
	m.mu.Lock()
	m.sessions[id] = s
	m.mu.Unlock()
	v := s.View()
	m.d.Sink.SessionChanged(v)
	return v, nil
}

func (m *Manager) Pages(id string) ([]store.Page, error) { return m.d.Store.ListPages(id) }

func (m *Manager) Send(ctx context.Context, id, text string) error {
	s, err := m.get(id)
	if err != nil {
		return err
	}
	return s.Send(ctx, text)
}

func (m *Manager) Interrupt(id string) error {
	s, err := m.get(id)
	if err != nil {
		return err
	}
	return s.Interrupt()
}

func (m *Manager) SetModel(id, model string) error {
	s, err := m.get(id)
	if err != nil {
		return err
	}
	return s.SetModel(model)
}

// ToggleModifier switches a modifier on or off for a session, keeping
// groups exclusive. It applies from the next turn.
func (m *Manager) ToggleModifier(id, modifierID string, on bool) (View, error) {
	s, err := m.get(id)
	if err != nil {
		return View{}, err
	}
	return s.ToggleModifier(modifierID, on)
}

// SetSortOrder places a session in the list (higher sorts first).
func (m *Manager) SetSortOrder(id string, order float64) (View, error) {
	s, err := m.get(id)
	if err != nil {
		return View{}, err
	}
	return s.SetSortOrder(order)
}

func (m *Manager) Rename(id, title string) (View, error) {
	s, err := m.get(id)
	if err != nil {
		return View{}, err
	}
	return s.Rename(title)
}

func (m *Manager) SetBookmark(sessionID, pageID string, on bool) (store.Page, error) {
	if err := m.d.Store.SetBookmark(pageID, on); err != nil {
		return store.Page{}, err
	}
	pages, err := m.d.Store.ListPages(sessionID)
	if err != nil {
		return store.Page{}, err
	}
	for _, p := range pages {
		if p.ID == pageID {
			m.d.Sink.PageChanged(p)
			return p, nil
		}
	}
	return store.Page{}, errors.New("page not found")
}

// Focus marks the session the user is looking at; its Unread clears.
func (m *Manager) Focus(id string) {
	m.mu.Lock()
	m.focused = id
	s := m.sessions[id]
	m.mu.Unlock()
	if s != nil {
		s.markRead()
	}
}

func (m *Manager) isFocused(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.focused == id
}

func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	s, ok := m.sessions[id]
	delete(m.sessions, id)
	m.mu.Unlock()
	if ok {
		s.stop()
	}
	return m.d.Store.DeleteSession(id)
}

// Models returns the models the CLI last reported, if any.
func (m *Manager) Models() []core.ModelInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]core.ModelInfo(nil), m.models...)
}

func (m *Manager) setModels(models []core.ModelInfo) {
	m.mu.Lock()
	m.models = models
	m.mu.Unlock()
	if b, err := json.Marshal(models); err == nil {
		_ = m.d.Store.SetSetting(modelsSetting, string(b))
	}
}

func (m *Manager) Usage() *core.UsageLimit {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.usage
}

func (m *Manager) setUsage(u core.UsageLimit) {
	m.mu.Lock()
	m.usage = &u
	m.mu.Unlock()
}

// Close stops every CLI process.
func (m *Manager) Close() {
	m.mu.Lock()
	all := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		all = append(all, s)
	}
	m.mu.Unlock()
	for _, s := range all {
		s.stop()
	}
}

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

	"uncli/internal/artifacts"
	"uncli/internal/core"
	"uncli/internal/profile"
	"uncli/internal/store"
)

// BinaryFunc resolves a provider's CLI binary to run and its version.
type BinaryFunc func(ctx context.Context, provider string) (path, version string, err error)

// HookServer gives a CLI process a way to ask UNCLI about tool calls
// (internal/hook): a handler for its calls, and the environment that leads
// its hooks to it.
type HookServer interface {
	Register(h func(ctx context.Context, event string, payload []byte) ([]byte, error)) (env map[string]string, unregister func())
}

type Deps struct {
	Store *store.Store
	// Adapter is the first provider, for sessions that don't name one;
	// Others are the rest. A session uses the one its record names.
	Adapter core.Adapter
	Others  []core.Adapter
	// Hooks carries hook calls for adapters that ask through one
	// (HookApprovals); nil: such sessions can't approve tool calls.
	Hooks   HookServer
	Runtime core.Runtime
	// Runtimes finds the runtime of a session that doesn't run locally, by
	// its runtime id and ref (a container profile); nil: local only.
	Runtimes   func(id, ref string) (core.Runtime, error)
	Profiles   *profile.Set
	Binary     BinaryFunc
	Sink       Sink
	ScratchDir string // chat sessions get <ScratchDir>/<session id>
	// Artifacts keeps the versions of what session types with artifacts
	// put in their artifacts folder; nil keeps none.
	Artifacts *artifacts.Store
	// Judge asks the app's decision model about commands UNCLI doesn't
	// recognise, and Unknown says what the user wants done with them
	// (UnknownModel, UnknownInside, UnknownAsk). DeciderKey names the
	// current decision model (core.DeciderInfo.Key), so answers cached from
	// another one are asked again. Any may be nil.
	Judge JudgeFunc
	// JudgeTools asks about several tools of one MCP server at once
	// (judging.go); nil judges each tool on its own as it is used.
	JudgeTools JudgeToolsFunc
	Unknown    func() string
	DeciderKey func() string
}

// RuntimeWSL is the runtime of sessions in a container (a WSL distro).
const RuntimeWSL = "wsl"

// runtime is where a session's CLI runs.
func (m *Manager) runtime(rec store.Session) (core.Runtime, error) {
	if rec.Runtime == "" || rec.Runtime == m.d.Runtime.ID() {
		return m.d.Runtime, nil
	}
	if m.d.Runtimes == nil {
		return nil, fmt.Errorf("this session runs in %s, which this UNCLI can't start", rec.Runtime)
	}
	return m.d.Runtimes(rec.Runtime, rec.RuntimeRef)
}

// adapterNamed is a provider's adapter; empty is the first provider.
func (m *Manager) adapterNamed(id string) (core.Adapter, error) {
	if id == "" || id == m.d.Adapter.ID() {
		return m.d.Adapter, nil
	}
	for _, a := range m.d.Others {
		if a.ID() == id {
			return a, nil
		}
	}
	return nil, fmt.Errorf("no AI provider %q", id)
}

// adapterOf is the adapter a session runs on. One whose provider this UNCLI
// no longer has keeps the first, and fails to start with a clear error.
func (m *Manager) adapterOf(rec store.Session) core.Adapter {
	if a, err := m.adapterNamed(rec.Adapter); err == nil {
		return a
	}
	return missingAdapter{m.d.Adapter, rec.Adapter}
}

// ErrNoCLI means the CLI isn't installed yet; the UI shows the setup screen.
var ErrNoCLI = errors.New("the CLI is not installed yet")

type Manager struct {
	d Deps

	mu       sync.Mutex
	sessions map[string]*Session
	focused  string
	models   []core.ModelInfo
	usage    *core.UsageLimit
	judge    judging

	// InterruptGrace is how long an interrupt waits for the CLI before the
	// process is killed instead.
	InterruptGrace time.Duration
}

const modelsSetting = "claude.models"

func NewManager(d Deps) (*Manager, error) {
	m := &Manager{d: d, sessions: map[string]*Session{}, InterruptGrace: 5 * time.Second,
		judge: judging{inFlight: map[string]bool{}, failed: map[string]bool{}}}
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
	return m.CreateIn(profileID, workdir, model, "")
}

// CreateIn creates a session whose CLI runs in a container profile's WSL
// distro (decision 0011); an empty container runs it on this machine.
func (m *Manager) CreateIn(profileID, workdir, model, container string) (View, error) {
	return m.CreateWith(NewSession{Profile: profileID, Workdir: workdir, Model: model, Container: container})
}

// NewSession is what a new session starts from.
type NewSession struct {
	Profile, Workdir, Model string
	Container               string // a container profile; empty: this machine
	Provider                string // the AI provider (an adapter's id); empty: the first
}

// CreateWith makes a new session on a provider. A profile's model is the
// first provider's; another starts on its CLI's own default unless given one.
func (m *Manager) CreateWith(n NewSession) (View, error) {
	profileID, workdir, model, container := n.Profile, n.Workdir, n.Model, n.Container
	p, ok := m.d.Profiles.Profile(profileID)
	if !ok {
		return View{}, fmt.Errorf("unknown profile %q", profileID)
	}
	ad, err := m.adapterNamed(n.Provider)
	if err != nil {
		return View{}, err
	}
	if model == "" && ad == m.d.Adapter {
		model = p.Model
	}
	if container != "" && ad != m.d.Adapter {
		return View{}, errors.New("containers can only run " + m.d.Adapter.ID() + " sessions for now")
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
		_ = os.MkdirAll(filepath.Join(workdir, artifacts.Folder), 0o755)
	}
	rec := store.Session{ID: id, Adapter: ad.ID(), Runtime: m.d.Runtime.ID(), ProfileID: p.ID,
		Workdir: workdir, Model: model, Modifiers: append([]string{}, p.ModifiersOn...)}
	if container != "" {
		if m.d.Runtimes == nil {
			return View{}, errors.New("containers aren't available here")
		}
		if _, err := m.d.Runtimes(RuntimeWSL, container); err != nil {
			return View{}, err
		}
		rec.Runtime, rec.RuntimeRef = RuntimeWSL, container
	}
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

// View is one session as the UI sees it.
func (m *Manager) View(id string) (View, error) {
	s, err := m.get(id)
	if err != nil {
		return View{}, err
	}
	return s.View(), nil
}

func (m *Manager) Pages(id string) ([]store.Page, error) { return m.d.Store.ListPages(id) }

func (m *Manager) Send(ctx context.Context, id, text string, files ...core.Attachment) error {
	s, err := m.get(id)
	if err != nil {
		return err
	}
	return s.Send(ctx, text, files)
}

// Answer gives the user's decision on a waiting request: allow (once),
// safe (and remember it, in scope "project" or "all") or deny.
func (m *Manager) Answer(id, requestID, decision, scope string) error {
	s, err := m.get(id)
	if err != nil {
		return err
	}
	return s.Answer(requestID, decision, scope)
}

// MarkSafe toggles "This is safe" on a waiting card, then looks again at
// every session's other waiting requests.
func (m *Manager) MarkSafe(id, requestID string, on bool, scope string) error {
	s, err := m.get(id)
	if err != nil {
		return err
	}
	if err := s.MarkSafe(requestID, on, scope); err != nil {
		return err
	}
	m.rejudgeAll()
	return nil
}

// SetUnattended turns a session's unattended mode on or off.
func (m *Manager) SetUnattended(id string, on bool) (View, error) {
	s, err := m.get(id)
	if err != nil {
		return View{}, err
	}
	return s.SetUnattended(on), nil
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
	return m.marked(sessionID, pageID, func(p *store.Page) { p.Bookmarked = on })
}

// SetPinned pins or unpins a page (Pinned lists them across sessions).
func (m *Manager) SetPinned(sessionID, pageID string, on bool) (store.Page, error) {
	if err := m.d.Store.SetPinned(pageID, on); err != nil {
		return store.Page{}, err
	}
	return m.marked(sessionID, pageID, func(p *store.Page) { p.Pinned = on })
}

// Pinned lists the pinned pages of every session, newest first.
func (m *Manager) Pinned() ([]store.PinnedPage, error) { return m.d.Store.Pinned() }

// marked applies a bookmark or pin to the session's open page too, when
// it is that page (the session saves and sends its own copy as the answer
// streams), and returns the page as stored.
func (m *Manager) marked(sessionID, pageID string, set func(*store.Page)) (store.Page, error) {
	if s, err := m.get(sessionID); err == nil {
		s.mu.Lock()
		if s.page != nil && s.page.ID == pageID {
			set(s.page)
		}
		s.mu.Unlock()
	}
	p, err := m.d.Store.Page(pageID)
	if err != nil {
		return store.Page{}, err
	}
	m.d.Sink.PageChanged(p)
	return p, nil
}

// Archive archives a session (on) or restores it. Archiving stops its CLI;
// it isn't deleted, and can be read and restored.
func (m *Manager) Archive(id string, on bool) (View, error) {
	s, err := m.get(id)
	if err != nil {
		return View{}, err
	}
	return s.Archive(on)
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

// Focused says whether the user is looking at a session: it is the one
// shown and UNCLI's window has focus.
func (m *Manager) Focused(id string) bool { return m.isFocused(id) }

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

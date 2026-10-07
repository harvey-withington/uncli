package session

import (
	"errors"
	"fmt"
	"os"

	"uncli/internal/core"
	"uncli/internal/store"
)

// Importing a conversation the CLI saved (core.TranscriptReader): it
// becomes a session whose provider id is the transcript's, so the next turn
// resumes it as any session does (--resume from its folder), with a page
// per turn. A session whose folder no longer exists is imported archived:
// it can be read, and restored once the folder is back.

// ImportedSession is what an import makes.
type ImportedSession struct {
	View        View `json:"session"`
	FolderGone  bool `json:"folderGone"`  // imported archived: its folder no longer exists
	PagesLoaded int  `json:"pagesLoaded"` // pages made from its turns
}

// Import makes a session from a transcript, of a session type and model.
func (m *Manager) Import(t core.Transcript, profileID, model string) (ImportedSession, error) {
	if t.Info.ID == "" || t.Info.Workdir == "" {
		return ImportedSession{}, errors.New("that conversation doesn't say where it ran")
	}
	if _, ok := m.d.Profiles.Profile(profileID); !ok {
		return ImportedSession{}, fmt.Errorf("unknown session type %q", profileID)
	}
	if v, ok := m.ByProvider(t.Info.ID); ok {
		return ImportedSession{View: v}, errors.New("that conversation is already in UNCLI")
	}
	st, err := os.Stat(t.Info.Workdir)
	gone := err != nil || !st.IsDir()
	rec := store.Session{ID: NewID(), Adapter: m.d.Adapter.ID(), Runtime: m.d.Runtime.ID(), ProfileID: profileID,
		Workdir: t.Info.Workdir, ProviderSID: t.Info.ID, CLIVersion: t.Info.CLIVersion, Model: model,
		Modifiers: []string{}, Archived: gone}
	if t.Info.FirstQuestion != "" {
		rec.Title = titleFrom(t.Info.FirstQuestion)
	}
	if err := m.d.Store.CreateSession(&rec); err != nil {
		return ImportedSession{}, err
	}
	for i, turn := range t.Turns {
		p := pageFrom(rec, i+1, turn, model)
		if err := m.d.Store.SavePage(&p); err != nil {
			_ = m.d.Store.DeleteSession(rec.ID)
			return ImportedSession{}, err
		}
	}
	s := newSession(m, rec)
	m.mu.Lock()
	m.sessions[rec.ID] = s
	m.mu.Unlock()
	v := s.View()
	m.d.Sink.SessionChanged(v)
	return ImportedSession{View: v, FolderGone: gone, PagesLoaded: len(t.Turns)}, nil
}

// ByProvider finds the session that holds a provider conversation.
func (m *Manager) ByProvider(providerSID string) (View, bool) {
	for _, v := range m.List() {
		if v.ProviderSID == providerSID {
			return v, true
		}
	}
	return View{}, false
}

func pageFrom(rec store.Session, seq int, t core.TranscriptTurn, model string) store.Page {
	p := store.Page{ID: NewID(), SessionID: rec.ID, Seq: seq, Question: t.Question, Model: model, Modifiers: []string{},
		AnswerMD: t.Answer, Status: "done", StartedAt: t.StartedAt, FinishedAt: t.FinishedAt,
		InputTokens: t.Usage.InputTokens, OutputTokens: t.Usage.OutputTokens, CacheRead: t.Usage.CacheRead, CacheWrite: t.Usage.CacheWrite,
		Trace: []store.TraceItem{}, TouchedFiles: []store.TouchedFile{}, Attachments: []store.PageAttachment{}, Artifacts: []store.ArtifactVersion{}}
	if t.FinishedAt > t.StartedAt {
		p.DurationMS = int(t.FinishedAt - t.StartedAt)
	}
	if t.Origin == OriginCLI {
		p.Origin = OriginCLI
	}
	if t.Error {
		p.Status, p.Error = "error", "The turn ended in an error."
	}
	for _, tl := range t.Tools {
		p.Trace = append(p.Trace, store.TraceItem{ID: tl.ID, Name: tl.Name, Summary: tl.Summary, Done: true, OK: tl.OK, Output: tl.Output})
	}
	for _, f := range t.Files {
		p.TouchedFiles = mergeTouched(p.TouchedFiles, store.TouchedFile{Path: f.Path, Line: f.Line, How: f.How, Added: f.Added, Removed: f.Removed})
	}
	return p
}

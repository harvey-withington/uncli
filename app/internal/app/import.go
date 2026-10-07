package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"uncli/internal/core"
	"uncli/internal/session"
)

// Importing conversations the CLI saved: from the terminal, or a UNCLI
// session that was deleted (its transcript outlives it). The adapter reads
// them (core.TranscriptReader); this lists them with what UNCLI knows about
// each, and imports one as a session.

// TranscriptEntry is a saved conversation as the import picker lists it.
type TranscriptEntry struct {
	core.TranscriptInfo
	SessionID  string `json:"sessionId,omitempty"` // already in UNCLI as this session
	FolderGone bool   `json:"folderGone"`          // its folder no longer exists
	FromChat   bool   `json:"fromChat"`            // it ran in a UNCLI chat folder: a deleted UNCLI chat
	Profile    string `json:"profile"`             // the session type it would be imported as
}

func (s *Service) transcriptReader() (core.TranscriptReader, core.TranscriptLocation, error) {
	r, ok := any(s.Adapter).(core.TranscriptReader)
	if !ok || !s.Adapter.Capabilities().Import {
		return nil, core.TranscriptLocation{}, errors.New("this CLI's conversations can't be imported")
	}
	if s.transcripts != nil {
		return r, *s.transcripts, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, core.TranscriptLocation{}, err
	}
	return r, core.TranscriptLocation{Home: home, ConfigDir: os.Getenv("CLAUDE_CONFIG_DIR")}, nil
}

// Transcripts lists the conversations the CLI has saved, newest first.
func (s *Service) Transcripts() ([]TranscriptEntry, error) {
	r, loc, err := s.transcriptReader()
	if err != nil {
		return nil, err
	}
	list, err := r.ListTranscripts(loc)
	if err != nil {
		return nil, err
	}
	have := map[string]string{}
	for _, v := range s.Sessions.List() {
		if v.ProviderSID != "" {
			have[v.ProviderSID] = v.ID
		}
	}
	out := make([]TranscriptEntry, 0, len(list))
	for _, info := range list {
		e := TranscriptEntry{TranscriptInfo: info, SessionID: have[info.ID], FromChat: s.inScratch(info.Workdir)}
		if st, err := os.Stat(info.Workdir); err != nil || !st.IsDir() {
			e.FolderGone = true
		}
		e.Profile = "code"
		if e.FromChat {
			e.Profile = "chat"
		}
		out = append(out, e)
	}
	return out, nil
}

// inScratch: a folder UNCLI made for a chat session.
func (s *Service) inScratch(dir string) bool {
	rel, err := filepath.Rel(s.Paths.Scratch, dir)
	return err == nil && rel != "." && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel)
}

// ImportTranscript imports a saved conversation as a session of the given
// type (empty: the type it would be guessed as).
func (s *Service) ImportTranscript(id, profileID string) (session.ImportedSession, error) {
	r, loc, err := s.transcriptReader()
	if err != nil {
		return session.ImportedSession{}, err
	}
	t, err := r.ReadTranscript(loc, id)
	if err != nil {
		return session.ImportedSession{}, err
	}
	if profileID == "" {
		profileID = "code"
		if s.inScratch(t.Info.Workdir) {
			profileID = "chat"
		}
	}
	p, ok := s.Profiles.Profile(profileID)
	if !ok {
		return session.ImportedSession{}, errors.New("unknown session type")
	}
	model := p.Model
	for i := len(t.Turns) - 1; i >= 0; i-- {
		if m := t.Turns[i].Model; m != "" {
			model = s.modelAlias(m, p.Model)
			break
		}
	}
	return s.Sessions.Import(t, profileID, model)
}

// modelAlias turns a full model id into the name the model picker uses
// ("claude-haiku-4-5-20251001" -> "haiku"), else keeps the fallback.
func (s *Service) modelAlias(full, fallback string) string {
	for _, m := range s.Sessions.Models() {
		if m.Resolved == full {
			return m.Value
		}
	}
	for _, family := range []string{"opus", "sonnet", "haiku"} {
		if strings.Contains(full, family) {
			return family
		}
	}
	return fallback
}

package session

import (
	"encoding/json"
	"slices"
)

// Turns cut off by UNCLI stopping: quitting (an update, a restart) or
// crashing with an answer in progress. They end as interrupted, as a turn
// the user stopped does, so which they were is kept apart: Close notes the
// sessions it cuts off, and pages still open at the next start (a crash, a
// forced quit) count too. The app can then carry them on (Preferences
// ResumeCutOff). A turn the user stopped is never among them.

const cutOffSetting = "sessions.cutoff" // JSON: session ids

// loadCutOff reads which sessions were cut off, before the open
// pages are closed, and forgets the note so it's read once.
func (m *Manager) loadCutOff() error {
	var ids []string
	if raw, _ := m.d.Store.Setting(cutOffSetting); raw != "" {
		_ = json.Unmarshal([]byte(raw), &ids)
	}
	open, err := m.d.Store.OpenPageSessions()
	if err != nil {
		return err
	}
	for _, id := range open {
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	m.cutOff = ids
	if len(ids) > 0 {
		return m.d.Store.SetSetting(cutOffSetting, "")
	}
	return nil
}

// CutOff is the sessions whose turn was cut off when UNCLI last stopped.
func (m *Manager) CutOff() []string { return slices.Clone(m.cutOff) }

// noteCutOff saves which sessions have a turn in progress, as UNCLI stops.
func (m *Manager) noteCutOff(all []*Session) {
	var ids []string
	for _, s := range all {
		s.mu.Lock()
		if s.page != nil {
			ids = append(ids, s.rec.ID)
		}
		s.mu.Unlock()
	}
	if len(ids) == 0 {
		return
	}
	b, _ := json.Marshal(ids)
	_ = m.d.Store.SetSetting(cutOffSetting, string(b))
}

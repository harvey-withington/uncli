package app

import "time"

// Turns UNCLI's last quit or crash cut off (session/cutoff.go) carry on
// as it starts again, when the user asked for it (Preferences.ResumeCutOff):
// each session is told why, in a message of its own, so it picks up where
// it was and the page says what happened. One at a time, a moment apart,
// so a dozen CLIs don't all start at once. An archived session is left.

const resumeText = "UNCLI restarted while you were working on this, and your last turn was cut off. Carry on where you left off."

const resumeGap = 2 * time.Second

// ResumeCutOff carries on the cut-off turns, if the user wants that; the
// app calls it once its window is up.
func (s *Service) ResumeCutOff() {
	if !s.Preferences().ResumeCutOff {
		return
	}
	first := true
	for _, id := range s.Sessions.CutOff() {
		v, err := s.Sessions.View(id)
		if err != nil || v.Session.Archived {
			continue
		}
		if !first {
			select {
			case <-s.life.Done():
				return
			case <-time.After(resumeGap):
			}
		}
		first = false
		// A session that can't start now (its CLI signed out, its container
		// gone) keeps its interrupted page, as without this.
		_ = s.Sessions.Send(s.life, id, resumeText)
	}
}

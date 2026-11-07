package session

import (
	"github.com/harvey-withington/uncli/app/internal/core"
	"github.com/harvey-withington/uncli/app/internal/store"
)

// A turn's changes are kept for the walkthrough (decision 0029): each
// tool's hunks, as its adapter reported them, in page_changes. A page's
// changes are capped, so a big rewrite doesn't swell the database; past
// the cap a file is listed without its text, as one a command changed is.
const (
	maxPageChanges   = 2 << 20   // bytes of a page's recorded hunks
	maxChangeRecord  = 512 << 10 // bytes of one tool's hunks
	changeLineAllows = 16 << 10  // longest line kept as it is
)

// changeOf is a touched file's hunks as they're stored, and their size.
func changeOf(f core.FileTouched) (store.FileChange, int) {
	c := store.FileChange{Path: f.Path, Hunks: make([]store.Hunk, 0, len(f.Hunks))}
	size := 0
	for _, h := range f.Hunks {
		lines := make([]string, len(h.Lines))
		for i, l := range h.Lines {
			if len(l) > changeLineAllows {
				l = l[:changeLineAllows] + "…"
			}
			lines[i] = l
			size += len(l) + 1
		}
		c.Hunks = append(c.Hunks, store.Hunk{OldStart: h.OldStart, NewStart: h.NewStart, Lines: lines})
	}
	return c, size
}

// changeBudget is what's left of a page's cap.
type changeBudget struct{ used int }

// take says whether a change of this size still fits, and counts it.
func (b *changeBudget) take(size int) bool {
	if size == 0 || size > maxChangeRecord || b.used+size > maxPageChanges {
		return false
	}
	b.used += size
	return true
}

// recordChange stores a touched file's hunks with the page, and says how
// many it kept (0 when there were none, or past the cap). Called with s.mu
// held, during the turn.
func (s *Session) recordChange(f core.FileTouched) int {
	if len(f.Hunks) == 0 || s.page == nil {
		return 0
	}
	if s.changesPage != s.page.ID {
		s.changesPage, s.changes = s.page.ID, changeBudget{}
	}
	c, size := changeOf(f)
	if !s.changes.take(size) {
		return 0
	}
	if err := s.m.d.Store.AddPageChange(s.rec.ID, s.page.ID, c); err != nil {
		return 0
	}
	return len(c.Hunks)
}

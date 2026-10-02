package clipfiles

import (
	"path/filepath"
	"testing"
)

// Reads whatever is on the clipboard without changing it: there may be
// files on it or not, but reading must work and give full paths.
func TestPathsReadsTheClipboard(t *testing.T) {
	got, err := Paths()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range got {
		if !filepath.IsAbs(p) {
			t.Errorf("not a full path: %q", p)
		}
	}
	t.Logf("%d file(s) on the clipboard", len(got))
}

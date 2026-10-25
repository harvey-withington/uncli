package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUploads(t *testing.T) {
	s := newTestService(t)
	up, err := s.SaveUpload(`..\..\evil/notes:.txt`, strings.NewReader("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if up.Name != "notes_.txt" || up.Kind != "text" || up.Size != 5 {
		t.Errorf("upload = %+v", up)
	}
	p, err := s.UploadPath(up.ID)
	if err != nil || filepath.Dir(filepath.Dir(p)) != s.uploadsDir() {
		t.Fatalf("path = %q, %v", p, err)
	}
	for _, bad := range []string{"", "../x", strings.Repeat("a", 24), up.ID + "0"} {
		if _, err := s.UploadPath(bad); err == nil {
			t.Errorf("%q found", bad)
		}
	}
	// Too big: refused and nothing kept.
	if _, err := s.SaveUpload("big.pdf", strings.NewReader(strings.Repeat("x", MaxUpload+1))); err == nil {
		t.Error("an upload over the limit was kept")
	}
	entries, _ := os.ReadDir(s.uploadsDir())
	if len(entries) != 1 {
		t.Errorf("%d uploads kept", len(entries))
	}
	// All uploads together at their limit: the next is refused.
	full := filepath.Join(s.uploadsDir(), "full", "big.bin")
	_ = os.MkdirAll(filepath.Dir(full), 0o700)
	f, err := os.Create(full)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(MaxUploads); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := s.SaveUpload("one-more.txt", strings.NewReader("x")); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Errorf("over the total = %v", err)
	}
}

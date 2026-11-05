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

// A file is copied as it's attached, so one gone by Send (a screenshot
// tool's temporary file, replaced by the next) still goes, under its own
// path; one that couldn't be copied is read at Send, as before.
func TestAttachedFilesAreCopiedAtOnce(t *testing.T) {
	s := newTestService(t)
	dir := t.TempDir()
	shot := filepath.Join(dir, "2026-11-05 11-26-07.png")
	png := []byte("\x89PNG\r\n\x1a\n first screenshot")
	if err := os.WriteFile(shot, png, 0o600); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(dir, "a folder")
	_ = os.Mkdir(folder, 0o700)
	infos := s.DescribeAttachments([]string{shot, folder})
	if infos[0].Upload == "" || infos[0].Path != shot || infos[0].Kind != "image" {
		t.Fatalf("screenshot = %+v", infos[0])
	}
	if infos[1].Upload != "" {
		t.Errorf("a folder was copied: %+v", infos[1])
	}
	if err := os.Remove(shot); err != nil {
		t.Fatal(err)
	}
	files, err := s.loadAttachments([]AttachmentRef{{Upload: infos[0].Upload, Path: shot}})
	if err != nil {
		t.Fatal(err)
	}
	if f := files[0]; string(f.Data) != string(png) || f.Path != shot || f.Name != "2026-11-05 11-26-07.png" || f.MediaType != "image/png" {
		t.Errorf("sent = %q %q %q %q", f.Name, f.Path, f.MediaType, f.Data)
	}
	// Without the copy, a file gone by Send says so.
	if _, err := s.loadAttachments([]AttachmentRef{{Path: shot}}); err == nil || !strings.Contains(err.Error(), "isn't there any more") {
		t.Errorf("gone = %v", err)
	}
}

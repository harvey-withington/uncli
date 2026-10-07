package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestThemeFile(t *testing.T) {
	s := &Service{Paths: Paths{Config: t.TempDir()}}
	th, err := s.Theme()
	if err != nil || th.Found {
		t.Fatalf("no file: %+v, %v", th, err)
	}
	src := "light:\n  bg: \"#f0f2f5\"\n  accent: \"#5e60ce\"\ndark:\n  bg: \"#18181b\"\n"
	if err := os.WriteFile(filepath.Join(s.Paths.Config, "theme.yaml"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	th, err = s.Theme()
	if err != nil || !th.Found || th.Light["accent"] != "#5e60ce" || th.Dark["bg"] != "#18181b" {
		t.Fatalf("file: %+v, %v", th, err)
	}
	if err := os.WriteFile(filepath.Join(s.Paths.Config, "theme.yaml"), []byte("light: [oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Theme(); err == nil {
		t.Error("a broken file should say so")
	}
}

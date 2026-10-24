package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListFolder(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []string{"beta", "Alpha", ".git"} {
		if err := os.Mkdir(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ListFolder(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Folders) != 2 || got.Folders[0].Name != "Alpha" || got.Folders[1].Path != filepath.Join(dir, "beta") {
		t.Errorf("folders = %+v", got.Folders)
	}
	if got.Parent != filepath.Dir(dir) {
		t.Errorf("parent = %q", got.Parent)
	}
	if _, err := ListFolder("relative/path"); err == nil {
		t.Error("a relative path was listed")
	}
	top, err := ListFolder("")
	home, _ := os.UserHomeDir()
	if err != nil || len(top.Folders) == 0 || top.Folders[0].Path != home {
		t.Errorf("top = %+v, %v", top, err)
	}
}

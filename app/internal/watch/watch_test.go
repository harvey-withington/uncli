package watch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func write(t *testing.T, root, rel, text string, age time.Duration) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	when := time.Now().Add(-age)
	_ = os.Chtimes(p, when, when)
}

// A search finds paths containing it, most recently changed first; folders
// nobody watches and binary files are left out.
func TestList(t *testing.T) {
	root := t.TempDir()
	write(t, root, "TODO.md", "old", 3*time.Hour)
	write(t, root, "docs/TODO-next.md", "newer", time.Hour)
	write(t, root, "plan/todo.txt", "newest", time.Minute)
	write(t, root, "README.md", "readme", 2*time.Hour)
	write(t, root, "node_modules/pkg/TODO.md", "skip", 0)
	write(t, root, ".git/TODO", "skip", 0)
	write(t, root, "logo.png", "\x89PNG", 0)

	got, err := List(root, "todo", 10)
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{}
	for _, e := range got {
		paths = append(paths, e.Path)
	}
	want := []string{"plan/todo.txt", "docs/TODO-next.md", "TODO.md"}
	if len(paths) != len(want) {
		t.Fatalf("paths = %v", paths)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("paths = %v, want %v", paths, want)
		}
	}
	all, _ := List(root, "", 2)
	if len(all) != 2 || all[0].Path != "plan/todo.txt" {
		t.Errorf("all = %+v", all)
	}
}

// A file is read when it changed; unchanged, nothing comes back; outside the
// folder is refused; missing, binary and huge files say so.
func TestRead(t *testing.T) {
	root := t.TempDir()
	write(t, root, "TODO.md", "# Todo\n- one", time.Minute)
	f, ok, err := Read(root, "TODO.md", 0)
	if err != nil || !ok || f.Text != "# Todo\n- one" || f.Modified == 0 {
		t.Fatalf("read = %+v %v %v", f, ok, err)
	}
	if _, ok, _ := Read(root, "TODO.md", f.Modified); ok {
		t.Error("unchanged: nothing to send")
	}
	write(t, root, "TODO.md", "# Todo\n- one\n- two", 0)
	if g, ok, _ := Read(root, "TODO.md", f.Modified); !ok || g.Text != "# Todo\n- one\n- two" {
		t.Errorf("changed = %+v %v", g, ok)
	}
	for _, bad := range []string{"../escape.md", `C:\Windows\win.ini`, ""} {
		if _, _, err := Read(root, bad, 0); err == nil {
			t.Errorf("%q should be refused", bad)
		}
	}
	if g, ok, _ := Read(root, "gone.md", 0); !ok || !g.Missing {
		t.Errorf("missing = %+v", g)
	}
	if _, ok, _ := Read(root, "gone.md", -1); ok {
		t.Error("still missing: nothing to send")
	}
	write(t, root, "data.txt", "a\x00b", 0)
	if g, _, _ := Read(root, "data.txt", 0); !g.Binary {
		t.Errorf("binary = %+v", g)
	}
	big := make([]byte, MaxSize+1)
	_ = os.WriteFile(filepath.Join(root, "big.log"), big, 0o644)
	if g, _, _ := Read(root, "big.log", 0); !g.TooBig {
		t.Errorf("too big = %+v", g)
	}
}

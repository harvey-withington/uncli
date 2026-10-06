package snapshot

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func write(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func kinds(changes []Change, root string) map[string]string {
	out := map[string]string{}
	for _, c := range changes {
		rel, _ := filepath.Rel(root, c.Path)
		out[filepath.ToSlash(rel)] = c.Kind
	}
	return out
}

func TestDiffInAPlainFolder(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "keep.txt"), "same")
	write(t, filepath.Join(root, "edit.txt"), "before")
	write(t, filepath.Join(root, "gone.txt"), "x")
	write(t, filepath.Join(root, "node_modules", "pkg", "index.js"), "x")
	write(t, filepath.Join(root, ".git-like", "x"), "x")
	before := Take(context.Background(), root)
	if !before.Complete {
		t.Fatal("snapshot incomplete")
	}

	write(t, filepath.Join(root, "edit.txt"), "after, longer")
	write(t, filepath.Join(root, "sub", "new.txt"), "new")
	_ = os.Remove(filepath.Join(root, "gone.txt"))
	write(t, filepath.Join(root, "node_modules", "pkg", "other.js"), "noise")
	write(t, filepath.Join(root, ".git-like", "y"), "noise")
	after := Take(context.Background(), root)

	got := kinds(Diff(before, after), root)
	want := map[string]string{"edit.txt": Changed, "sub/new.txt": Added, "gone.txt": Deleted}
	if len(got) != len(want) {
		t.Fatalf("changes = %v", got)
	}
	for p, k := range want {
		if got[p] != k {
			t.Errorf("%s: %q, want %q (all: %v)", p, got[p], k, got)
		}
	}
}

func TestSameSizeEditFoundByTime(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "a.txt")
	write(t, p, "aaaa")
	before := Take(context.Background(), root)
	write(t, p, "bbbb")
	later := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(p, later, later)
	if got := kinds(Diff(before, Take(context.Background(), root)), root); got["a.txt"] != Changed {
		t.Errorf("changes = %v", got)
	}
}

func TestGitRepoHonoursIgnore(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("no git")
	}
	root := t.TempDir()
	if out, err := exec.Command(git, "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	write(t, filepath.Join(root, ".gitignore"), "ignored/\n*.log\n")
	write(t, filepath.Join(root, "src", "main.go"), "package main")
	before := Take(context.Background(), root)
	write(t, filepath.Join(root, "src", "main.go"), "package main // changed")
	write(t, filepath.Join(root, "ignored", "big.bin"), "x")
	write(t, filepath.Join(root, "run.log"), "x")
	write(t, filepath.Join(root, "untracked.txt"), "x")
	got := kinds(Diff(before, Take(context.Background(), root)), root)
	if got["src/main.go"] != Changed || got["untracked.txt"] != Added || len(got) != 2 {
		t.Errorf("changes = %v", got)
	}
}

func TestIncompleteFindsNothing(t *testing.T) {
	root := t.TempDir()
	a := Take(context.Background(), root)
	b := Snapshot{Root: root} // as if the folder were too big
	if d := Diff(a, b); d != nil {
		t.Errorf("diff = %v", d)
	}
	if d := Diff(a, Take(context.Background(), t.TempDir())); d != nil {
		t.Errorf("diff across folders = %v", d)
	}
}

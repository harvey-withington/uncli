package gitinfo

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHead(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	write(t, filepath.Join(repo, ".git", "HEAD"), "ref: refs/heads/feat/remote-servers\n")
	sub := filepath.Join(repo, "app", "internal")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	r := NewReader()
	if h, ok := r.Head(sub); !ok || h.Branch != "feat/remote-servers" {
		t.Errorf("from a subfolder = %+v %v", h, ok)
	}

	// A checkout elsewhere is read again; a detached HEAD gives its commit.
	head := filepath.Join(repo, ".git", "HEAD")
	write(t, head, "0d40146a7a86e2d1c0ffee0000000000000000aa\n")
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(head, later, later); err != nil {
		t.Fatal(err)
	}
	if h, ok := r.Head(repo); !ok || h.Branch != "" || h.Commit != "0d40146" {
		t.Errorf("detached = %+v %v", h, ok)
	}

	// A worktree's .git file points at its own HEAD.
	wt := filepath.Join(root, "wt")
	write(t, filepath.Join(wt, ".git"), "gitdir: ../repo/.git/worktrees/wt\n")
	write(t, filepath.Join(repo, ".git", "worktrees", "wt", "HEAD"), "ref: refs/heads/main\n")
	if h, ok := r.Head(wt); !ok || h.Branch != "main" {
		t.Errorf("worktree = %+v %v", h, ok)
	}

	// Not a repository, until one is made there and a while has passed.
	plain := filepath.Join(root, "plain")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	r.now = func() time.Time { return now }
	if _, ok := r.Head(plain); ok {
		t.Error("a plain folder has a branch")
	}
	write(t, filepath.Join(plain, ".git", "HEAD"), "ref: refs/heads/main\n")
	if _, ok := r.Head(plain); ok {
		t.Error("a miss was looked at again at once")
	}
	now = now.Add(missAge + time.Second)
	if h, ok := r.Head(plain); !ok || h.Branch != "main" {
		t.Errorf("after git init = %+v %v", h, ok)
	}
	if _, ok := r.Head(""); ok {
		t.Error("no folder has a branch")
	}
}

package app

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

// A host's own UNCLI (decision 0027) leaves the desktop's machine-wide
// parts to the host: updates, starting with Windows, containers, hooks.
func TestEmbeddedLeavesTheMachineAlone(t *testing.T) {
	dir := t.TempDir()
	s, err := NewWith(Paths{Config: filepath.Join(dir, "c"), Cache: filepath.Join(dir, "k"), Scratch: filepath.Join(dir, "s")}, nopEmitter{}, Options{Embedded: true})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if _, err := s.CheckForUpdates(ctx); !errors.Is(err, errEmbedded) {
		t.Errorf("CheckForUpdates: %v", err)
	}
	if _, err := s.InstallUpdate(ctx); !errors.Is(err, errEmbedded) {
		t.Errorf("InstallUpdate: %v", err)
	}
	p := s.Preferences()
	p.StartWithWindows = true
	if _, err := s.SetPreferences(p); !errors.Is(err, errEmbedded) {
		t.Errorf("starting with Windows: %v", err)
	}
	if err := s.BuildContainer("code"); !errors.Is(err, errEmbedded) {
		t.Errorf("BuildContainer: %v", err)
	}
	if info := s.Containers(ctx); len(info.Containers) != 0 || len(info.Bases) != 0 {
		t.Errorf("containers offered: %+v", info)
	}
	if _, err := s.runtimeFor("wsl", "code"); err == nil {
		t.Error("a container runtime was started")
	}
}

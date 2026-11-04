package datadir

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

var bruv = Owner{Kind: KindHost, Name: "BRUV"}

func TestOneUNCLIAtATime(t *testing.T) {
	dir := t.TempDir()
	release, err := Claim(dir, bruv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Claim(dir, bruv); !errors.Is(err, ErrInUse) {
		t.Fatalf("a second claim got %v", err)
	}
	release()
	again, err := Claim(dir, bruv)
	if err != nil {
		t.Fatalf("after release: %v", err)
	}
	again()
}

func TestAFolderStaysWithItsOwner(t *testing.T) {
	dir := t.TempDir()
	release, err := Claim(dir, bruv)
	if err != nil {
		t.Fatal(err)
	}
	release()
	for _, o := range []Owner{{Kind: KindHost, Name: "Another app"}, {Kind: KindDesktop}} {
		if _, err := Claim(dir, o); !errors.Is(err, ErrNotYours) {
			t.Errorf("%s got %v", o, err)
		}
	}
	// The same host, its name in another case, is still the owner.
	if r, err := Claim(dir, Owner{Kind: KindHost, Name: "bruv"}); err != nil {
		t.Errorf("the owner was refused: %v", err)
	} else {
		r()
	}
}

func TestAnUnmarkedUNCLIIsTheDesktopApps(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "uncli.db"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Claim(dir, bruv); !errors.Is(err, ErrNotYours) {
		t.Fatalf("a host took an unmarked UNCLI: %v", err)
	}
	r, err := Claim(dir, Owner{Kind: KindDesktop})
	if err != nil {
		t.Fatalf("the desktop app was refused its own folder: %v", err)
	}
	r()
	if _, err := Claim(dir, bruv); !errors.Is(err, ErrNotYours) {
		t.Fatalf("a host took the desktop app's folder: %v", err)
	}
}

func TestInside(t *testing.T) {
	base := filepath.Join(t.TempDir(), "uncli")
	for path, want := range map[string]bool{
		base:                                        true,
		filepath.Join(base, "host"):                 true,
		filepath.Join(base, "..", "uncli"):          true,
		filepath.Join(base, "..", "uncli-x"):        false,
		filepath.Dir(base):                          false,
		filepath.Join(base, "..", "other", "uncli"): false,
	} {
		if got := Inside(path, base); got != want {
			t.Errorf("Inside(%s) = %v", path, got)
		}
	}
}

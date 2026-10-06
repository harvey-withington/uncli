package notify

import (
	"testing"
	"unicode/utf16"
)

func TestClip(t *testing.T) {
	if got := clip("short", 10); got != "short" {
		t.Errorf("clip kept %q", got)
	}
	got := clip("abcdefghij", 5)
	if got != "abcd…" {
		t.Errorf("clip = %q", got)
	}
	// An emoji is two UTF-16 units: never split in half.
	got = clip("ab😀cd", 4)
	if n := len(utf16.Encode([]rune(got))); n > 4 || got != "ab…" {
		t.Errorf("clip = %q (%d units)", got, n)
	}
}

func TestNoneShowsNothing(t *testing.T) {
	var n Notifier = None{}
	if err := n.Show(Note{Title: "x"}); err != nil {
		t.Fatal(err)
	}
	n.Close()
}

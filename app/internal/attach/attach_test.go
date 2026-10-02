package attach

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDescribeKinds(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		path, kind, media string
	}{
		{write(t, dir, "shot.PNG", []byte("\x89PNG")), Image, "image/png"},
		{write(t, dir, "photo.jpeg", []byte{0xff, 0xd8}), Image, "image/jpeg"},
		{write(t, dir, "brief.pdf", []byte("%PDF-1.4")), PDF, "application/pdf"},
		{write(t, dir, "notes.md", []byte("# Notes\nCafé ✓\n")), Text, "text/plain"},
		{write(t, dir, "main.go", []byte("package main\n")), Text, "text/plain"},
		{write(t, dir, "Makefile", []byte("all:\n\tgo build\n")), Text, "text/plain"},
		{write(t, dir, "tool.exe", []byte("MZ\x90\x00\x03")), Unsupported, ""},
		{write(t, dir, "latin1.txt", []byte("caf\xe9 au lait")), Unsupported, ""},
		{dir, Folder, ""},
		{filepath.Join(dir, "gone.txt"), Unsupported, ""},
	}
	for _, c := range cases {
		got := Describe(c.path)
		if got.Kind != c.kind || got.MediaType != c.media {
			t.Errorf("%s: kind %q media %q, want %q %q (%s)", filepath.Base(c.path), got.Kind, got.MediaType, c.kind, c.media, got.Reason)
		}
		if got.Attachable() == (got.Reason != "") {
			t.Errorf("%s: attachable %v with reason %q", filepath.Base(c.path), got.Attachable(), got.Reason)
		}
	}
}

// A text sample may end part-way through a character; that's still text.
func TestDescribeTextCutMidRune(t *testing.T) {
	dir := t.TempDir()
	body := append(bytes.Repeat([]byte("a"), 8<<10-1), []byte("é")...) // é straddles the 8 KB sample
	if got := Describe(write(t, dir, "long.txt", body)); got.Kind != Text {
		t.Errorf("kind = %q (%s)", got.Kind, got.Reason)
	}
}

func TestDescribeLimits(t *testing.T) {
	dir := t.TempDir()
	big := write(t, dir, "huge.txt", bytes.Repeat([]byte("x"), MaxText+1))
	got := Describe(big)
	if got.Kind != Unsupported || !strings.Contains(got.Reason, "512 KB limit for text files") {
		t.Errorf("got %+v", got)
	}
	if got := Describe(write(t, dir, "ok.txt", bytes.Repeat([]byte("x"), MaxText))); got.Kind != Text {
		t.Errorf("at the limit: %+v", got)
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, "notes.txt", []byte("hello"))
	a, err := Load(p)
	if err != nil || a.Name != "notes.txt" || a.Path != p || a.MediaType != "text/plain" || string(a.Data) != "hello" {
		t.Errorf("load = %+v, %v", a, err)
	}
	if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), "folders can't be attached") {
		t.Errorf("folder: %v", err)
	}
}

func TestFromData(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n0000")
	a, err := FromData("", "", png)
	if err != nil || a.MediaType != "image/png" || a.Name != "Pasted image" || a.Path != "" {
		t.Errorf("png = %+v, %v", a, err)
	}
	if _, err := FromData("x", "text/plain", []byte("hi")); err == nil {
		t.Error("only images can be pasted as data")
	}
	if _, err := FromData("x", "image/png", bytes.Repeat([]byte{1}, MaxImage+1)); err == nil {
		t.Error("an oversized image must be refused")
	}
}

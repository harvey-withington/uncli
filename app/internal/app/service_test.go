package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"uncli/internal/core"
	"uncli/internal/store"
)

func TestLoginProblem(t *testing.T) {
	// 2.1.285 prints "Login successful." and exits 0 even for a bad code.
	out := "Paste code here if prompted > Login successful.\nInvalid code. Please make sure the full code was copied.\n"
	if got := loginProblem(out); got != "Invalid code. Please make sure the full code was copied." {
		t.Errorf("got %q", got)
	}
	if got := loginProblem("Paste code here if prompted > Login successful.\n"); got != "" {
		t.Errorf("got %q", got)
	}
}

func TestNewSessionChoicesAreRemembered(t *testing.T) {
	dir := t.TempDir()
	paths := Paths{Config: filepath.Join(dir, "cfg"), Cache: filepath.Join(dir, "cache"), Scratch: filepath.Join(dir, "scratch")}
	svc, err := New(paths, nopEmitter{})
	if err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	if _, _, err := svc.CreateSession("code", repo, "haiku"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.CreateSession("chat", "", "sonnet"); err != nil {
		t.Fatal(err)
	}
	svc.Close()

	svc, err = New(paths, nopEmitter{})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	last := svc.Bootstrap().LastNew
	if last.ProfileID != "chat" || last.Models["code"] != "haiku" || last.Models["chat"] != "sonnet" || last.Folders["code"] != repo {
		t.Errorf("remembered = %+v", last)
	}
	if _, ok := last.Folders["chat"]; ok {
		t.Error("chat's scratch folder is per session and shouldn't be remembered")
	}
}

type nopEmitter struct{}

func (nopEmitter) Emit(string, any) {}

func TestParseSections(t *testing.T) {
	res := core.TextResult{Structured: json.RawMessage(`{"sections":[{"block":0,"title":" Overview "},{"block":3,"title":"Later"},{"block":2,"title":"Out of order"},{"block":9,"title":"Past the end"},{"block":4,"title":""}]}`)}
	got, err := parseSections(res, 5)
	if err != nil || len(got) != 2 || got[0] != (store.OutlineSection{Block: 0, Title: "Overview"}) || got[1].Block != 3 {
		t.Errorf("sections = %+v, %v", got, err)
	}
	if _, err := parseSections(core.TextResult{Text: "not json"}, 5); err == nil {
		t.Error("junk should fail")
	}
	plain, err := parseSections(core.TextResult{Text: `{"sections":[{"block":1,"title":"Plain"}]}`}, 3)
	if err != nil || len(plain) != 1 {
		t.Errorf("plain JSON answer = %+v, %v", plain, err)
	}
}

func TestPreferences(t *testing.T) {
	dir := t.TempDir()
	svc, err := New(Paths{Config: filepath.Join(dir, "c"), Cache: filepath.Join(dir, "k"), Scratch: filepath.Join(dir, "s")}, nopEmitter{})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	if p := svc.Preferences(); p.QuickTaskModel != (ModelRef{"claude", "haiku"}) || p.AutoSummary != SummaryOff {
		t.Errorf("defaults = %+v", p)
	}
	if _, err := svc.SetPreferences(Preferences{QuickTaskModel: ModelRef{"qwen-local", "qwen3"}}); err == nil {
		t.Error("an unknown provider must be refused")
	}
	if _, err := svc.SetPreferences(Preferences{QuickTaskModel: ModelRef{"claude", "haiku"}, AutoSummary: "sometimes"}); err == nil {
		t.Error("an unknown summary setting must be refused")
	}
	p, err := svc.SetPreferences(Preferences{QuickTaskModel: ModelRef{"claude", "sonnet"}, AutoSummary: SummaryAlways})
	if err != nil || p.QuickTaskModel.Model != "sonnet" || p.AutoSummary != SummaryAlways {
		t.Errorf("saved = %+v, %v", p, err)
	}
	// The old on/off switch carries over: on meant long answers.
	if err := svc.Store.SetSetting(settingPrefs, `{"quickTaskModel":{"provider":"claude","model":"haiku"},"autoSummarise":true}`); err != nil {
		t.Fatal(err)
	}
	if p := svc.Preferences(); p.AutoSummary != SummaryLong {
		t.Errorf("migrated = %+v", p)
	}
}

func TestSectionKinds(t *testing.T) {
	got, err := parseSections(core.TextResult{Structured: json.RawMessage(`{"sections":[{"block":0,"title":"Intro","kind":"overview"},{"block":1,"title":"Odd","kind":"poetry"}]}`)}, 2)
	if err != nil || got[0].Kind != "overview" || got[1].Kind != "" {
		t.Errorf("kinds = %+v, %v", got, err)
	}
	// The frontend's list must match: it picks the icons.
	src, err := os.ReadFile(filepath.Join("..", "..", "frontend", "src", "lib", "sections.ts"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?s)SECTION_KINDS = \[(.*?)\] as const`).FindSubmatch(src)
	if m == nil {
		t.Fatal("SECTION_KINDS not found in sections.ts")
	}
	ts := regexp.MustCompile(`'([a-z]+)'`).FindAllSubmatch(m[1], -1)
	if len(ts) != len(sectionKinds) {
		t.Fatalf("frontend has %d kinds, backend %d", len(ts), len(sectionKinds))
	}
	for i, k := range sectionKinds {
		if string(ts[i][1]) != k.ID {
			t.Errorf("kind %d: frontend %q, backend %q", i, ts[i][1], k.ID)
		}
	}
	if !strings.Contains(string(outlineSchema), `"enum":["overview",`) {
		t.Errorf("schema = %s", outlineSchema)
	}
}

func TestDescribePaths(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "notes.md")
	os.WriteFile(file, []byte("x"), 0o644)
	got := DescribePaths([]string{dir, file, filepath.Join(dir, "gone")})
	if len(got) != 2 || !got[0].IsDir || got[0].Dir != dir || got[1].IsDir || got[1].Dir != dir {
		t.Errorf("paths = %+v", got)
	}
}

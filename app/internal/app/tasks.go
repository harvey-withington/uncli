package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"uncli/internal/core"
	"uncli/internal/ide"
	"uncli/internal/runtime/local"
	"uncli/internal/session"
	"uncli/internal/store"
)

// Quick tasks are one-off text jobs outside any session: page summaries
// now, things like commit messages later. They all use one model the user
// picks (provider and model), so a light or local model can take them.

type ModelRef struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type Preferences struct {
	QuickTaskModel ModelRef `json:"quickTaskModel"` // the model for every quick task
	AutoSummary    string   `json:"autoSummary"`    // which answers summarise themselves as they finish: off, long or always
	// UnknownCommands is what Ask mode does with a command UNCLI doesn't
	// recognise: model (the quick-task model judges it), inside (it runs if
	// it stays in the session folder) or ask.
	UnknownCommands string `json:"unknownCommands"`
	// DecisionModel answers fixed-answer questions such as "is this safe?"
	// (decide.go); the quick-task model unless set.
	DecisionModel DecisionModel `json:"decisionModel"`
	// Notifications says which desktop notifications to show (notify.go):
	// all, approvals or off.
	Notifications string `json:"notifications"`
	// Editor opens files from sessions that link to an IDE (files.go): a
	// preset id, auto (the first installed) or custom (EditorCommand).
	Editor        string `json:"editor"`
	EditorCommand string `json:"editorCommand,omitempty"`
}

// Automatic summaries: none, answers long enough to need one (the UI
// decides what's long), or every answer of more than one block.
const (
	SummaryOff    = "off"
	SummaryLong   = "long"
	SummaryAlways = "always"
)

func validSummary(v string) bool {
	return v == SummaryOff || v == SummaryLong || v == SummaryAlways
}

const settingPrefs = "prefs"

func defaultPreferences() Preferences {
	return Preferences{QuickTaskModel: ModelRef{Provider: "claude", Model: "haiku"}, AutoSummary: SummaryOff, UnknownCommands: session.UnknownModel,
		DecisionModel: DecisionModel{Provider: DeciderQuickTask}, Notifications: NotifyAll, Editor: ide.Auto}
}

func (s *Service) Preferences() Preferences {
	saved := struct {
		Preferences
		AutoSummarise bool `json:"autoSummarise"` // before AutoSummary: on meant long answers
	}{Preferences: defaultPreferences()}
	saved.AutoSummary = "" // unset until read, so the old switch can fill it
	if raw, _ := s.Store.Setting(settingPrefs); raw != "" {
		_ = json.Unmarshal([]byte(raw), &saved)
	}
	p := saved.Preferences
	if p.QuickTaskModel.Provider == "" || p.QuickTaskModel.Model == "" {
		p.QuickTaskModel = defaultPreferences().QuickTaskModel
	}
	if !validSummary(p.AutoSummary) {
		p.AutoSummary = SummaryOff
		if saved.AutoSummarise {
			p.AutoSummary = SummaryLong
		}
	}
	if !session.ValidUnknown(p.UnknownCommands) {
		p.UnknownCommands = session.UnknownModel
	}
	if p.DecisionModel.Provider == "" || validDecisionModel(p.DecisionModel) != nil {
		p.DecisionModel = defaultPreferences().DecisionModel
	}
	if !validNotify(p.Notifications) {
		p.Notifications = NotifyAll
	}
	if validEditor(p.Editor, p.EditorCommand) != nil {
		p.Editor, p.EditorCommand = ide.Auto, ""
	}
	return p
}

func (s *Service) SetPreferences(p Preferences) (Preferences, error) {
	if _, err := s.textTasker(p.QuickTaskModel.Provider); err != nil {
		return s.Preferences(), err
	}
	if strings.TrimSpace(p.QuickTaskModel.Model) == "" {
		return s.Preferences(), errors.New("choose a model for quick tasks")
	}
	if !validSummary(p.AutoSummary) {
		return s.Preferences(), fmt.Errorf("unknown summary setting %q", p.AutoSummary)
	}
	if p.UnknownCommands == "" {
		p.UnknownCommands = session.UnknownModel
	}
	if !session.ValidUnknown(p.UnknownCommands) {
		return s.Preferences(), fmt.Errorf("unknown setting for unrecognised commands %q", p.UnknownCommands)
	}
	if p.DecisionModel.Provider == "" {
		p.DecisionModel.Provider = DeciderQuickTask
	}
	if err := validDecisionModel(p.DecisionModel); err != nil {
		return s.Preferences(), err
	}
	if p.Notifications == "" {
		p.Notifications = NotifyAll
	}
	if !validNotify(p.Notifications) {
		return s.Preferences(), fmt.Errorf("unknown notification setting %q", p.Notifications)
	}
	if p.Editor == "" {
		p.Editor = ide.Auto
	}
	if err := validEditor(p.Editor, p.EditorCommand); err != nil {
		return s.Preferences(), err
	}
	b, err := json.Marshal(p)
	if err != nil {
		return s.Preferences(), err
	}
	if err := s.Store.SetSetting(settingPrefs, string(b)); err != nil {
		return s.Preferences(), err
	}
	return s.Preferences(), nil
}

func (s *Service) textTasker(provider string) (core.TextTasker, error) {
	if provider == s.Adapter.ID() {
		return s.Adapter, nil
	}
	return nil, fmt.Errorf("no provider %q for quick tasks", provider)
}

// RunTextTask runs a quick task on the user's chosen model.
func (s *Service) RunTextTask(ctx context.Context, t core.TextTask) (core.TextResult, ModelRef, error) {
	ref := s.Preferences().QuickTaskModel
	tasker, err := s.textTasker(ref.Provider)
	if err != nil {
		return core.TextResult{}, ref, err
	}
	bin, _, err := s.binary(ctx)
	if err != nil {
		return core.TextResult{}, ref, err
	}
	t.Model = ref.Model
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	// An empty folder of its own: from any other the CLI would read that
	// folder's CLAUDE.md and memory into the task.
	dir := filepath.Join(s.Paths.Cache, "quick-tasks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return core.TextResult{}, ref, err
	}
	out, runErr := local.RunInputIn(ctx, dir, tasker.TextTaskCommand(bin, t), t.Prompt)
	res, err := tasker.ParseTextTask(out)
	if err != nil {
		if runErr != nil && len(out) == 0 {
			return core.TextResult{}, ref, runErr
		}
		return core.TextResult{}, ref, err
	}
	return res, ref, nil
}

// sectionKinds is what a section of an answer can be; the outline shows
// an icon for each. Kept in step with SECTION_KINDS in
// frontend/src/lib/sections.ts (a test checks).
var sectionKinds = []struct{ ID, Meaning string }{
	{"overview", "introduces the topic or sets the context"},
	{"commentary", "general explanation or discussion"},
	{"analysis", "compares, evaluates, diagnoses or reasons about causes"},
	{"steps", "instructions or a procedure to follow"},
	{"code", "mostly code"},
	{"data", "tables, figures, results or numbers"},
	{"example", "a worked example or sample"},
	{"tip", "a recommendation, hint or best practice"},
	{"warning", "a caveat, risk or something to be careful about"},
	{"conclusion", "a summary, verdict or next steps"},
}

func validKind(k string) bool {
	for _, sk := range sectionKinds {
		if sk.ID == k {
			return true
		}
	}
	return false
}

var outlineSchema = func() json.RawMessage {
	ids := make([]string, len(sectionKinds))
	for i, k := range sectionKinds {
		ids[i] = k.ID
	}
	enum, _ := json.Marshal(ids)
	return json.RawMessage(`{"type":"object","properties":{"sections":{"type":"array","items":{"type":"object","properties":{"block":{"type":"integer"},"title":{"type":"string"},"kind":{"type":"string","enum":` + string(enum) + `}},"required":["block","title","kind"]}}},"required":["sections"]}`)
}()

const (
	maxBlockChars  = 300   // per block in the prompt
	maxPromptChars = 40000 // keeps a summary cheap on very long answers
)

// SummarisePage asks the quick-task model for a table of contents of a
// page's answer. blocks are the answer's top-level markdown blocks as the
// UI numbers them (code blocks described, not quoted), so each section
// title anchors to a block the UI can scroll to.
func (s *Service) SummarisePage(ctx context.Context, sessionID, pageID string, blocks []string) (store.Page, error) {
	if len(blocks) == 0 {
		return store.Page{}, errors.New("this answer is empty")
	}
	var b strings.Builder
	b.WriteString("Make a table of contents for the answer below so a reader can jump around it. Give between 3 and 12 sections in reading order, fewer if the answer is short. Each title is at most 6 words, in the answer's language. \"block\" is the number of the block where that section starts; the first section starts at block 0. \"kind\" says what the section is, one of:\n")
	for _, k := range sectionKinds {
		fmt.Fprintf(&b, "- %s: %s\n", k.ID, k.Meaning)
	}
	b.WriteString("\n")
	for i, text := range blocks {
		line := fmt.Sprintf("[%d] %s\n", i, clip(strings.Join(strings.Fields(text), " "), maxBlockChars))
		if b.Len()+len(line) > maxPromptChars {
			fmt.Fprintf(&b, "[%d…%d] (rest of the answer omitted)\n", i, len(blocks)-1)
			break
		}
		b.WriteString(line)
	}
	res, ref, err := s.RunTextTask(ctx, core.TextTask{
		System: "You write short, plain tables of contents for answers. Reply only with the requested JSON.",
		Prompt: b.String(),
		Schema: outlineSchema,
	})
	if err != nil {
		return store.Page{}, fmt.Errorf("couldn't summarise this page: %w", err)
	}
	sections, err := parseSections(res, len(blocks))
	if err != nil {
		return store.Page{}, fmt.Errorf("couldn't summarise this page: %w", err)
	}
	o := &store.PageOutline{Provider: ref.Provider, Model: ref.Model, Sections: sections, CostUSD: res.CostUSD, At: time.Now().UnixMilli()}
	if err := s.Store.SetPageOutline(pageID, o); err != nil {
		return store.Page{}, err
	}
	pages, err := s.Store.ListPages(sessionID)
	if err != nil {
		return store.Page{}, err
	}
	for _, p := range pages {
		if p.ID == pageID {
			s.emit.Emit(EvtPage, p)
			return p, nil
		}
	}
	return store.Page{}, errors.New("page not found")
}

// parseSections keeps the model's sections that point at real blocks, in
// order, one per block.
func parseSections(res core.TextResult, nBlocks int) ([]store.OutlineSection, error) {
	raw := res.Structured
	if len(raw) == 0 {
		raw = json.RawMessage(res.Text) // a provider without structured output may answer in plain JSON
	}
	var v struct {
		Sections []store.OutlineSection `json:"sections"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, errors.New("the model's answer wasn't the expected JSON")
	}
	out := []store.OutlineSection{}
	last := -1
	for _, sec := range v.Sections {
		title := clip(strings.Join(strings.Fields(sec.Title), " "), 80)
		if sec.Block <= last || sec.Block < 0 || sec.Block >= nBlocks || title == "" {
			continue
		}
		kind := sec.Kind
		if !validKind(kind) {
			kind = "" // the UI guesses
		}
		out = append(out, store.OutlineSection{Block: sec.Block, Title: title, Kind: kind})
		last = sec.Block
		if len(out) == 20 {
			break
		}
	}
	if len(out) == 0 {
		return nil, errors.New("the model gave no usable sections")
	}
	return out, nil
}

func clip(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}

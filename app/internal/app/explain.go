package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/harvey-withington/uncli/app/internal/core"
	"github.com/harvey-withington/uncli/app/internal/store"
)

// Explain a page's changes (decision 0029): a call of its own, outside the
// session, so it doesn't send the whole conversation again. It gets the
// page's changes, each place with an id, the page's question and answer
// (where the agent usually says what it's doing and why), and earlier pages
// only when asked. The model answers with a summary and a note per place,
// by id; notes for places that don't exist are dropped, and no code is
// taken from it.

// ExplainOptions are the choices behind Explain.
type ExplainOptions struct {
	Model   string `json:"model"`   // of the first provider; "" for the session's, else the quick-task model
	Context string `json:"context"` // page (the default) | pages | session
	Pages   int    `json:"pages"`   // pages: how many before this one
	Detail  string `json:"detail"`  // brief (the default) | thorough
}

const (
	maxExplainPrompt  = 160000 // characters; the changes come first, earlier pages give way
	maxExplainAnswer  = 12000  // of the page's own answer
	maxEarlierAnswer  = 2500   // of each earlier page's answer
	maxExplainPlace   = 8000   // of one changed place
	explainWordsBrief = 50
	explainWordsFull  = 150
)

var explainSchema = json.RawMessage(`{"type":"object","properties":{"summary":{"type":"string"},"notes":{"type":"array","items":{"type":"object","properties":{"place":{"type":"string"},"text":{"type":"string"}},"required":["place","text"]}}},"required":["summary","notes"]}`)

// placeID names a changed place: the change's index in PageChanges and the
// hunk's within it, as the UI's deck numbers them.
func placeID(change, hunk int) string { return fmt.Sprintf("c%dh%d", change, hunk) }

// explainPrompt is what the model is sent, and the ids of the places in it.
func explainPrompt(pages []store.Page, at int, changes []store.FileChange, workdir string, o ExplainOptions) (string, []string) {
	words := explainWordsBrief
	if o.Detail == "thorough" {
		words = explainWordsFull
	}
	var places strings.Builder
	var ids []string
	for n, c := range changes {
		for k, h := range c.Hunks {
			id := placeID(n, k)
			ids = append(ids, id)
			body := clip(strings.Join(h.Lines, "\n"), maxExplainPlace)
			fmt.Fprintf(&places, "### %s: %s (from line %d)\n```diff\n%s\n```\n\n", id, relTo(workdir, c.Path), h.NewStart, body)
		}
	}
	page := pages[at]
	var b strings.Builder
	fmt.Fprintf(&b, "An AI coding agent made the changes below during one turn of a conversation. Explain them to a developer reviewing them.\n\n"+
		"Give a summary of what the turn changed and why, in two or three sentences. Then a note for each changed place, by its id (as in \"c0h1\"), saying what it does and why, in plain words, in %d words at most. "+
		"A place that only makes sense with another can say so briefly. Take the reasons from the conversation where it gives them; don't invent reasons it doesn't give. "+
		"In the diffs, lines starting with + were added and - removed.\n\n", words)
	var earlier []store.Page
	switch o.Context {
	case "session":
		earlier = pages[:at]
	case "pages":
		earlier = pages[max(0, at-max(o.Pages, 1)):at]
	}
	room := maxExplainPrompt - places.Len() - 2*maxExplainAnswer
	var prior strings.Builder
	for i := len(earlier) - 1; i >= 0; i-- { // newest first, so what's cut is the oldest
		p := earlier[i]
		entry := fmt.Sprintf("Question: %s\nAnswer: %s\n\n", clip(p.Question, 1000), clip(p.AnswerMD, maxEarlierAnswer))
		if prior.Len()+len(entry) > room {
			break
		}
		prior.WriteString(entry)
	}
	if prior.Len() > 0 {
		b.WriteString("## Earlier in the conversation (latest first)\n\n")
		b.WriteString(prior.String())
	}
	fmt.Fprintf(&b, "## The turn\n\nQuestion: %s\n\nAnswer: %s\n\n## The changes\n\n%s", clip(page.Question, 4000), clip(page.AnswerMD, maxExplainAnswer), places.String())
	return b.String(), ids
}

// relTo is a path relative to the session folder, with slashes, when it's
// inside it.
func relTo(workdir, path string) string {
	w := strings.TrimRight(strings.ReplaceAll(workdir, "\\", "/"), "/") + "/"
	p := strings.ReplaceAll(path, "\\", "/")
	if workdir != "" && len(p) > len(w) && strings.EqualFold(p[:len(w)], w) {
		return p[len(w):]
	}
	return p
}

// explainInput gathers what the prompt needs.
func (s *Service) explainInput(sessionID, pageID string) ([]store.Page, int, []store.FileChange, store.Session, error) {
	rec, err := s.Store.GetSession(sessionID)
	if err != nil {
		return nil, 0, nil, rec, err
	}
	pages, err := s.Store.ListPages(sessionID)
	if err != nil {
		return nil, 0, nil, rec, err
	}
	at := slices.IndexFunc(pages, func(p store.Page) bool { return p.ID == pageID })
	if at < 0 {
		return nil, 0, nil, rec, errors.New("page not found")
	}
	changes, err := s.Store.PageChanges(sessionID, pageID)
	if err != nil {
		return nil, 0, nil, rec, err
	}
	if len(changes) == 0 {
		return nil, 0, nil, rec, errors.New("this page has no recorded changes to explain")
	}
	return pages, at, changes, rec, nil
}

// ExplainSize is roughly how many tokens an explanation would send, for the
// options to show before it starts.
func (s *Service) ExplainSize(sessionID, pageID string, o ExplainOptions) (int, error) {
	pages, at, changes, rec, err := s.explainInput(sessionID, pageID)
	if err != nil {
		return 0, err
	}
	prompt, _ := explainPrompt(pages, at, changes, rec.Workdir, o)
	return len(prompt) / 4, nil
}

// ExplainPage asks a model to explain a page's changes, and stores the
// explanation with the page.
func (s *Service) ExplainPage(ctx context.Context, sessionID, pageID string, o ExplainOptions) (store.Page, error) {
	pages, at, changes, rec, err := s.explainInput(sessionID, pageID)
	if err != nil {
		return store.Page{}, err
	}
	if o.Context != "pages" && o.Context != "session" {
		o.Context = "page"
	}
	if o.Detail != "thorough" {
		o.Detail = "brief"
	}
	ref := ModelRef{Provider: s.Adapter.ID(), Model: o.Model}
	if ref.Model == "" {
		if rec.Adapter == s.Adapter.ID() || rec.Adapter == "" {
			ref.Model = rec.Model
		} else {
			ref = s.Preferences().QuickTaskModel
		}
	}
	prompt, ids := explainPrompt(pages, at, changes, rec.Workdir, o)
	res, ref, err := s.runTextTaskOn(ctx, ref, core.TextTask{
		System: "You explain code changes to developers reviewing them, plainly and briefly. Reply only with the requested JSON.",
		Prompt: prompt,
		Schema: explainSchema,
	})
	if err != nil {
		return store.Page{}, fmt.Errorf("couldn't explain these changes: %w", err)
	}
	e, err := parseExplanation(res, ids)
	if err != nil {
		return store.Page{}, fmt.Errorf("couldn't explain these changes: %w", err)
	}
	e.Model, e.Context, e.Detail, e.CostUSD, e.At = ref.Model, o.Context, o.Detail, res.CostUSD, time.Now().UnixMilli()
	if o.Context == "pages" {
		e.Pages = max(o.Pages, 1)
	}
	if err := s.Store.SetPageExplanation(pageID, e); err != nil {
		return store.Page{}, err
	}
	p, err := s.Store.Page(pageID)
	if err != nil {
		return store.Page{}, err
	}
	s.emit.Emit(EvtPage, p)
	return p, nil
}

// parseExplanation keeps the model's notes for places that exist, once
// each, in the places' order.
func parseExplanation(res core.TextResult, ids []string) (*store.PageExplanation, error) {
	raw := res.Structured
	if len(raw) == 0 {
		raw = json.RawMessage(res.Text)
	}
	var v struct {
		Summary string              `json:"summary"`
		Notes   []store.ExplainNote `json:"notes"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("the model's answer wasn't the expected JSON: %w", err)
	}
	byID := map[string]string{}
	for _, n := range v.Notes {
		if t := strings.TrimSpace(n.Text); t != "" && slices.Contains(ids, n.Place) && byID[n.Place] == "" {
			byID[n.Place] = t
		}
	}
	e := &store.PageExplanation{Summary: strings.TrimSpace(v.Summary), Notes: []store.ExplainNote{}}
	for _, id := range ids {
		if t := byID[id]; t != "" {
			e.Notes = append(e.Notes, store.ExplainNote{Place: id, Text: t})
		}
	}
	if e.Summary == "" && len(e.Notes) == 0 {
		return nil, errors.New("the model's answer was empty")
	}
	return e, nil
}

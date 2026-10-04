//go:build integration

// Package integration drives the real Claude CLI through UNCLI's app
// service: the pinned download, three concurrent sessions, restart and
// resume, a live model switch, modifiers, interrupt and permissions. It
// uses the machine's Claude sign-in and real (small, Haiku) turns, so it is
// not part of npm test. Run with: npm run test:integration
package integration

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"uncli/internal/app"
	"uncli/internal/session"
	"uncli/internal/store"
)

type recorder struct {
	mu     sync.Mutex
	states map[string][]session.State
}

func (r *recorder) Emit(name string, data any) {
	if v, ok := data.(session.View); ok && name == app.EvtSessionView {
		r.mu.Lock()
		s := r.states[v.ID]
		if len(s) == 0 || s[len(s)-1] != v.State {
			r.states[v.ID] = append(s, v.State)
		}
		r.mu.Unlock()
	}
}

func (r *recorder) seen(id string) []session.State {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.states[id])
}

func paths(t *testing.T, config string) app.Paths {
	t.Helper()
	def, err := app.DefaultPaths()
	if err != nil {
		t.Fatal(err)
	}
	// The real cache, so the download is shared with the app; everything
	// else is throwaway.
	return app.Paths{Config: config, Cache: def.Cache, Scratch: filepath.Join(config, "scratch")}
}

func open(t *testing.T, config string) (*app.Service, *recorder) {
	t.Helper()
	rec := &recorder{states: map[string][]session.State{}}
	svc, err := app.New(paths(t, config), rec)
	if err != nil {
		t.Fatal(err)
	}
	return svc, rec
}

func waitIdle(t *testing.T, svc *app.Service, ids ...string) {
	t.Helper()
	deadline := time.Now().Add(4 * time.Minute)
	for time.Now().Before(deadline) {
		busy := false
		for _, v := range svc.Sessions.List() {
			if slices.Contains(ids, v.ID) && v.Busy {
				busy = true
			}
		}
		if !busy {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("sessions %v never finished", ids)
}

func send(t *testing.T, svc *app.Service, id, text string) store.Page {
	t.Helper()
	if err := svc.Sessions.Send(context.Background(), id, text); err != nil {
		t.Fatalf("send %q: %v", text, err)
	}
	waitIdle(t, svc, id)
	return last(t, svc, id)
}

func last(t *testing.T, svc *app.Service, id string) store.Page {
	t.Helper()
	pages, err := svc.Sessions.Pages(id)
	if err != nil || len(pages) == 0 {
		t.Fatalf("pages: %v", err)
	}
	return pages[len(pages)-1]
}

func view(svc *app.Service, id string) session.View {
	for _, v := range svc.Sessions.List() {
		if v.ID == id {
			return v
		}
	}
	return session.View{}
}

func TestPhase1EndToEnd(t *testing.T) {
	config := t.TempDir()
	svc, rec := open(t, config)

	// The pinned CLI downloads, verifies and reports a signed-in user.
	ctx := context.Background()
	st, err := svc.InstallCLI(ctx)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !st.Installed || st.Version != svc.Installer.Pinned() {
		t.Fatalf("status after install = %+v", st)
	}
	if !st.LoggedIn {
		t.Skip("the CLI is not signed in on this machine; sign in and rerun")
	}
	t.Logf("CLI %s installed, signed in (%s)", st.Version, st.Subscription)

	cowork := t.TempDir()
	os.WriteFile(filepath.Join(cowork, "notes.md"), []byte("# Notes\n\nThe launch date is 14 March.\n"), 0o644)
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}

	chat, err := svc.Sessions.Create("chat", "", "haiku")
	if err != nil {
		t.Fatal(err)
	}
	co, err := svc.Sessions.Create("cowork", cowork, "haiku")
	if err != nil {
		t.Fatal(err)
	}
	code, err := svc.Sessions.Create("code", repo, "haiku")
	if err != nil {
		t.Fatal(err)
	}
	svc.Sessions.Focus(code.ID)

	t.Run("three sessions at once", func(t *testing.T) {
		start := time.Now()
		for id, q := range map[string]string{
			chat.ID: "Remember the word banjo. Reply with just OK.",
			co.ID:   "What is the launch date in notes.md? Answer in five words or fewer.",
			code.ID: "Run `git status` and tell me in one sentence whether the repository has any commits.",
		} {
			if err := svc.Sessions.Send(ctx, id, q); err != nil {
				t.Fatal(err)
			}
		}
		// All three are busy at the same time.
		busy := 0
		for _, v := range svc.Sessions.List() {
			if v.Busy {
				busy++
			}
		}
		if busy != 3 {
			t.Errorf("busy sessions = %d, want 3", busy)
		}
		waitIdle(t, svc, chat.ID, co.ID, code.ID)
		t.Logf("three turns took %s", time.Since(start).Round(time.Second))

		for _, id := range []string{chat.ID, co.ID, code.ID} {
			p := last(t, svc, id)
			if p.Status != "done" || p.AnswerMD == "" || p.OutputTokens == 0 || p.CostUSD <= 0 {
				t.Errorf("%s page = status %s, answer %q, error %q", id, p.Status, p.AnswerMD, p.Error)
			}
			states := rec.seen(id)
			if !slices.Contains(states, session.Thinking) {
				t.Errorf("%s never showed thinking: %v", id, states)
			}
		}
		if p := last(t, svc, co.ID); !strings.Contains(p.AnswerMD, "14") {
			t.Errorf("co-work answer should read the folder: %q", p.AnswerMD)
		}
		if p := last(t, svc, code.ID); len(p.Trace) == 0 || !slices.ContainsFunc(p.Trace, func(i store.TraceItem) bool { return strings.Contains(i.Summary, "git status") && !i.Denied }) {
			t.Errorf("code session should have run git status: %+v", p.Trace)
		}
		if st := view(svc, code.ID).State; st != session.Idle {
			t.Errorf("focused session state = %s, want idle", st)
		}
		if st := view(svc, chat.ID).State; st != session.Unread {
			t.Errorf("unfocused session state = %s, want unread", st)
		}
	})

	t.Run("denied tools show in the trace", func(t *testing.T) {
		// Ask mode asks only about risky commands: git clean deletes
		// untracked files for good, so it waits on a card; the user denies it.
		probe := filepath.Join(repo, "probe.txt")
		os.WriteFile(probe, []byte("keep me"), 0o644)
		if err := svc.Sessions.Send(context.Background(), code.ID, "Use Bash to run exactly `git clean -fdx` and report the result, or say DENIED if you can't. Don't try any other way."); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(3 * time.Minute)
		for len(view(svc, code.ID).Approvals) == 0 && view(svc, code.ID).Busy && time.Now().Before(deadline) {
			time.Sleep(100 * time.Millisecond)
		}
		if a := view(svc, code.ID).Approvals; len(a) > 0 {
			if err := svc.Sessions.Answer(code.ID, a[0].RequestID, session.Deny, ""); err != nil {
				t.Fatal(err)
			}
		}
		waitIdle(t, svc, code.ID)
		p := last(t, svc, code.ID)
		if !slices.ContainsFunc(p.Trace, func(i store.TraceItem) bool { return i.Denied }) {
			t.Errorf("git clean is risky and the user denied it: %+v", p.Trace)
		}
		if _, err := os.Stat(probe); err != nil {
			t.Error("the denied command ran anyway")
		}
	})

	t.Run("model switch continues the conversation", func(t *testing.T) {
		if err := svc.Sessions.SetModel(chat.ID, "sonnet"); err != nil {
			t.Fatal(err)
		}
		p := send(t, svc, chat.ID, "What word did I ask you to remember? Then name the model you are, in one line.")
		if p.Model != "sonnet" || !strings.Contains(strings.ToLower(p.AnswerMD), "banjo") || !strings.Contains(strings.ToLower(p.AnswerMD), "sonnet") {
			t.Errorf("after switch: model %s, answer %q", p.Model, p.AnswerMD)
		}
		svc.Sessions.SetModel(chat.ID, "haiku")
	})

	t.Run("efficiency mode on and off", func(t *testing.T) {
		q := "Explain what a hash map is."
		normal := send(t, svc, co.ID, q)
		if _, err := svc.Sessions.ToggleModifier(co.ID, "efficiency", true); err != nil {
			t.Fatal(err)
		}
		lean := send(t, svc, co.ID, q)
		if _, err := svc.Sessions.ToggleModifier(co.ID, "efficiency", false); err != nil {
			t.Fatal(err)
		}
		after := send(t, svc, co.ID, q)
		if !slices.Equal(lean.Modifiers, []string{"efficiency"}) || !strings.Contains(lean.Directives, "minimum tokens") {
			t.Errorf("efficiency page = %+v", lean)
		}
		if !strings.Contains(after.Directives, "no longer applies") || len(after.Modifiers) != 0 {
			t.Errorf("switched-off page directives = %q", after.Directives)
		}
		t.Logf("answer lengths: normal %d, efficiency %d, after %d chars", len(normal.AnswerMD), len(lean.AnswerMD), len(after.AnswerMD))
		if len(lean.AnswerMD) >= len(normal.AnswerMD) {
			t.Errorf("efficiency answer (%d) should be shorter than normal (%d)", len(lean.AnswerMD), len(normal.AnswerMD))
		}
	})

	t.Run("interrupt keeps the session", func(t *testing.T) {
		if err := svc.Sessions.Send(ctx, chat.ID, "Write a 500-word story about a lighthouse keeper."); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(60 * time.Second)
		for view(svc, chat.ID).State != session.Writing && time.Now().Before(deadline) {
			time.Sleep(50 * time.Millisecond)
		}
		if err := svc.Sessions.Interrupt(chat.ID); err != nil {
			t.Fatal(err)
		}
		waitIdle(t, svc, chat.ID)
		if p := last(t, svc, chat.ID); p.Status != "interrupted" {
			t.Errorf("interrupted page status = %s", p.Status)
		}
		p := send(t, svc, chat.ID, "In one short sentence: what were you writing about?")
		if p.Status != "done" || !strings.Contains(strings.ToLower(p.AnswerMD), "lighthouse") {
			t.Errorf("after interrupt: %+v", p)
		}
	})

	// Bookmark a page, then quit.
	pages, _ := svc.Sessions.Pages(chat.ID)
	if _, err := svc.Sessions.SetBookmark(chat.ID, pages[0].ID, true); err != nil {
		t.Fatal(err)
	}
	before := len(pages)
	svc.Close()

	t.Run("relaunch restores and continues", func(t *testing.T) {
		svc2, _ := open(t, config)
		defer svc2.Close()
		list := svc2.Sessions.List()
		if len(list) != 3 {
			t.Fatalf("restored %d sessions", len(list))
		}
		pages, _ := svc2.Sessions.Pages(chat.ID)
		if len(pages) != before || !pages[0].Bookmarked {
			t.Fatalf("restored pages = %d (want %d), first bookmarked %v", len(pages), before, pages[0].Bookmarked)
		}
		p := send(t, svc2, chat.ID, "What word did I ask you to remember at the very start? One word.")
		if !strings.Contains(strings.ToLower(p.AnswerMD), "banjo") {
			t.Errorf("after relaunch the conversation should continue: %q", p.AnswerMD)
		}
	})
}

// Sign-in against an empty config folder: the CLI prints a link and waits
// for a code. On a machine that is already signed in elsewhere, 2.1.285
// completes the sign-in even with a wrong code (and still says "Invalid
// code"), so the check is that UNCLI's verdict matches "auth status".
func TestSignInFlow(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir()) // removed with its copied credentials
	svc, _ := open(t, t.TempDir())
	defer svc.Close()
	ctx := context.Background()
	if _, err := svc.InstallCLI(ctx); err != nil {
		t.Fatal(err)
	}
	if st := svc.CLIStatus(ctx, true); st.LoggedIn || st.Error != "" {
		t.Fatalf("empty config should read as signed out, got %+v", st)
	}
	start, err := svc.SignIn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if start.SignedIn {
		t.Log("the CLI signed in on its own (this machine is signed in elsewhere)")
		return
	}
	if !strings.HasPrefix(start.URL, "https://") || !strings.Contains(start.URL, "oauth/authorize") {
		t.Fatalf("sign-in link = %q", start.URL)
	}
	st, err := svc.SubmitLoginCode(ctx, "not-a-real-code")
	if (err == nil) != st.LoggedIn {
		t.Fatalf("verdict and status disagree: %+v %v", st, err)
	}
	t.Logf("signed in: %v (%v)", st.LoggedIn, err)
}

// VS Code's debugger auto-attach puts NODE_OPTIONS into every terminal; the
// CLI must not inherit it (with it, every CLI command exits 1 silently).
func TestDebuggerNodeOptionsDontLeak(t *testing.T) {
	t.Setenv("NODE_OPTIONS", `--require "C:/no/such/bootloader.js" --inspect-publish-uid=http`)
	svc, _ := open(t, t.TempDir())
	defer svc.Close()
	ctx := context.Background()
	if _, err := svc.InstallCLI(ctx); err != nil {
		t.Fatal(err)
	}
	if st := svc.CLIStatus(ctx, true); st.Error != "" || !st.LoggedIn {
		t.Fatalf("status with a debugger NODE_OPTIONS = %+v", st)
	}
}

// A page summary from the quick-task model: titles anchored to real blocks.
func TestSummarisePage(t *testing.T) {
	svc, _ := open(t, t.TempDir())
	defer svc.Close()
	ctx := context.Background()
	if _, err := svc.InstallCLI(ctx); err != nil {
		t.Fatal(err)
	}
	svc.Store.CreateSession(&store.Session{ID: "s", Adapter: "claude", Runtime: "local", ProfileID: "chat", Workdir: t.TempDir(), Model: "haiku"})
	svc.Store.SavePage(&store.Page{ID: "p", SessionID: "s", Seq: 1, Question: "Plan a weekend in Lisbon", Model: "haiku", Status: "done"})
	blocks := []string{
		"Here's a relaxed two-day plan for Lisbon.",
		"Saturday morning: walk Alfama before the crowds and stop at the Miradouro de Santa Luzia.",
		"Lunch at a tasca; grilled sardines in season.",
		"(code block: text, 3 lines)",
		"Sunday: Belém for pastéis de nata, then the Jerónimos Monastery (book ahead).",
		"Afternoon at LX Factory; sunset at Cais do Sodré.",
		"Budget: about €110 per person for food, transport and entry fees.",
	}
	p, err := svc.SummarisePage(ctx, "s", "p", blocks)
	if err != nil {
		t.Fatal(err)
	}
	o := p.Outline
	if o == nil || o.Provider != "claude" || o.Model != "haiku" || len(o.Sections) < 2 || o.Sections[0].Block != 0 {
		t.Fatalf("outline = %+v", o)
	}
	for _, s := range o.Sections {
		if s.Block < 0 || s.Block >= len(blocks) || s.Title == "" || s.Kind == "" {
			t.Errorf("bad section %+v", s)
		}
	}
	t.Logf("sections: %+v (cost $%.4f)", o.Sections, o.CostUSD)
}

// Files dropped or pasted go to the real CLI as content: two images (one
// from disk, one pasted as data), a PDF and a text file, in a chat session,
// which has no tool to read files, so the answer can only come from the
// attachments themselves.
func TestAttachments(t *testing.T) {
	svc, _ := open(t, t.TempDir())
	defer svc.Close()
	ctx := context.Background()
	if _, err := svc.InstallCLI(ctx); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	write := func(name string, data []byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	solid := func(c color.Color) []byte {
		img := image.NewRGBA(image.Rect(0, 0, 32, 32))
		draw.Draw(img, img.Bounds(), &image.Uniform{c}, image.Point{}, draw.Src)
		var b bytes.Buffer
		_ = png.Encode(&b, img)
		return b.Bytes()
	}
	rec, err := os.ReadFile(filepath.Join("..", "..", "testdata", "streams", "claude", "2.1.285", "document-input.in.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var in struct {
		Message struct {
			Content []struct{ Source struct{ Data string } }
		}
	}
	if err := json.Unmarshal(rec, &in); err != nil {
		t.Fatal(err)
	}
	pdf, _ := base64.StdEncoding.DecodeString(in.Message.Content[0].Source.Data) // says PELICAN

	v, err := svc.Sessions.Create("chat", "", "haiku")
	if err != nil {
		t.Fatal(err)
	}
	refs := []app.AttachmentRef{
		{Path: write("first.png", solid(color.RGBA{220, 0, 0, 255}))},
		{Name: "Pasted image", MediaType: "image/png", Data: base64.StdEncoding.EncodeToString(solid(color.RGBA{0, 0, 220, 255}))},
		{Path: write("brief.pdf", pdf)},
		{Path: write("notes.md", []byte("# Notes\n\nSecond code word: MARMALADE.\n"))},
	}
	q := "Reply on one line: the colour of each of the two images in order, then the code word in brief.pdf, then the one in notes.md."
	if err := svc.Send(ctx, v.ID, q, refs); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, svc, v.ID)
	p := last(t, svc, v.ID)
	ans := strings.ToLower(p.AnswerMD)
	for _, want := range []string{"red", "blue", "pelican", "marmalade"} {
		if !strings.Contains(ans, want) {
			t.Errorf("answer lacks %q: %s", want, p.AnswerMD)
		}
	}
	if len(p.Attachments) != 4 || p.Attachments[1].Name != "Pasted image" || p.Attachments[1].Path != "" || p.Attachments[3].MediaType != "text/plain" {
		t.Errorf("attachments = %+v", p.Attachments)
	}
	// A folder can't go.
	if err := svc.Send(ctx, v.ID, "x", []app.AttachmentRef{{Path: dir}}); err == nil || !strings.Contains(err.Error(), "folders can't be attached") {
		t.Errorf("folder: %v", err)
	}
	t.Logf("answer: %s (cost $%.4f)", p.AnswerMD, p.CostUSD)
}

// Approvals against the real CLI: a command outside the Code profile's
// allowlist waits on a card; allowed, it runs; denied, the model is told;
// with a project rule, it runs without asking.
func TestApprovals(t *testing.T) {
	svc, _ := open(t, t.TempDir())
	defer svc.Close()
	ctx := context.Background()
	if _, err := svc.InstallCLI(ctx); err != nil {
		t.Fatal(err)
	}
	askUnknown(t, svc) // node -e can't be placed: with "ask", it prompts
	v, err := svc.Sessions.Create("code", t.TempDir(), "haiku")
	if err != nil {
		t.Fatal(err)
	}
	ask := func(expr string) {
		q := `Use the Bash tool to run exactly: node -e "console.log(` + expr + `)" and then reply with only its output, or the word DENIED if it was refused.`
		if err := svc.Sessions.Send(ctx, v.ID, q); err != nil {
			t.Fatal(err)
		}
	}
	waitCard := func() session.Approval {
		deadline := time.Now().Add(3 * time.Minute)
		for time.Now().Before(deadline) {
			if a := view(svc, v.ID).Approvals; len(a) > 0 {
				return a[0]
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Fatal("no approval card")
		return session.Approval{}
	}

	ask("6*7")
	a := waitCard()
	if a.Tool != "Bash" || !strings.Contains(string(a.Input), "node -e") || len(a.Learn) != 0 || a.Why[0].Fixed != session.FixedInline {
		t.Fatalf("inline code can only be allowed once: %+v", a)
	}
	if view(svc, v.ID).State != session.NeedsApproval {
		t.Errorf("state = %s", view(svc, v.ID).State)
	}
	if err := svc.Sessions.Answer(v.ID, a.RequestID, session.Allow, ""); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, svc, v.ID)
	if p := last(t, svc, v.ID); !strings.Contains(p.AnswerMD, "42") {
		t.Errorf("allowed answer = %q", p.AnswerMD)
	}

	ask("5*5")
	a = waitCard()
	if err := svc.Sessions.Answer(v.ID, a.RequestID, session.Deny, ""); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, svc, v.ID)
	p := last(t, svc, v.ID)
	if strings.Contains(p.AnswerMD, "25") || !slices.ContainsFunc(p.Trace, func(i store.TraceItem) bool { return i.Denied }) {
		t.Errorf("denied answer = %q, trace = %+v", p.AnswerMD, p.Trace)
	}

	// "This is safe" remembers the class, so the same command runs on its
	// own the next time. git clean -n only lists what it would delete.
	clean := func() {
		if err := svc.Sessions.Send(ctx, v.ID, "Use the Bash tool to run exactly: git clean -n  and then reply DONE."); err != nil {
			t.Fatal(err)
		}
	}
	clean()
	a = waitCard()
	if len(a.Learn) != 1 || a.Learn[0].Words != "git clean" {
		t.Fatalf("card = %+v", a)
	}
	if err := svc.Sessions.Answer(v.ID, a.RequestID, session.Safe, session.ScopeProject); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, svc, v.ID)
	clean()
	waitIdle(t, svc, v.ID)
	if p := last(t, svc, v.ID); !slices.ContainsFunc(p.Trace, func(i store.TraceItem) bool { return i.Approved == "listed" }) {
		t.Errorf("the learned command should run on its own: %+v", p.Trace)
	}
	t.Logf("cost: allowed, denied and learned turns done")
}

// notesServer is a small MCP server with annotated and unannotated tools.
const notesServer = `import { createInterface } from "node:readline";
const tools = [
  { name: "read_note", description: "Read a note.", inputSchema: { type: "object", properties: { name: { type: "string" } } }, annotations: { readOnlyHint: true } },
  { name: "delete_note", description: "Delete a note.", inputSchema: { type: "object", properties: { name: { type: "string" } } }, annotations: { destructiveHint: true } },
  { name: "touch_note", description: "Create or update a note.", inputSchema: { type: "object", properties: { name: { type: "string" } } } },
];
const send = m => process.stdout.write(JSON.stringify(m) + "\n");
createInterface({ input: process.stdin }).on("line", l => {
  let m; try { m = JSON.parse(l) } catch { return }
  if (m.id === undefined) return;
  if (m.method === "initialize") return send({ jsonrpc: "2.0", id: m.id, result: { protocolVersion: m.params?.protocolVersion ?? "2025-06-18", capabilities: { tools: {} }, serverInfo: { name: "notes", version: "1.0.0" } } });
  if (m.method === "tools/list") return send({ jsonrpc: "2.0", id: m.id, result: { tools } });
  if (m.method === "tools/call") return send({ jsonrpc: "2.0", id: m.id, result: { content: [{ type: "text", text: "ok " + m.params.name }] } });
  send({ jsonrpc: "2.0", id: m.id, error: { code: -32601, message: "no such method" } });
});
`

// Prompt levels against the real CLI and a real MCP server: Always prompts
// for everything but the read tool, Never runs everything, a tool taught
// safe stops prompting, and Always with Unattended declines all but reading.
func TestSessionModes(t *testing.T) {
	config := t.TempDir()
	dir := t.TempDir()
	server := filepath.Join(dir, "notes.mjs")
	os.WriteFile(server, []byte(notesServer), 0o644)
	cfg, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{"notes": map[string]any{"command": "node", "args": []string{filepath.ToSlash(server)}}}})
	os.WriteFile(filepath.Join(dir, "mcp.json"), cfg, 0o644)
	os.WriteFile(filepath.Join(config, "profiles.yaml"), []byte(`profiles:
  - id: notes
    label: Notes
    folder: pick
    model: haiku
    tools: [Read]
    isolated: true
    permission_mode: default
    mcp_config: ["`+filepath.ToSlash(filepath.Join(dir, "mcp.json"))+`"]
`), 0o644)
	svc, _ := open(t, config)
	defer svc.Close()
	ctx := context.Background()
	if _, err := svc.InstallCLI(ctx); err != nil {
		t.Fatal(err)
	}
	askUnknown(t, svc) // touch_note has no annotations: with "ask", Ask mode asks about it
	v, err := svc.Sessions.Create("notes", t.TempDir(), "haiku")
	if err != nil {
		t.Fatal(err)
	}
	const q = "Call these notes tools one after another, each with name x: read_note, then touch_note, then delete_note. " +
		"Call all three even if one is refused. Then reply with one word per tool: OK or REFUSED."

	// run sends q in a mode and answers cards (touch allowed, delete denied);
	// it returns the tools that came to the user, and the page.
	run := func(mode string) ([]string, store.Page) {
		t.Helper()
		if _, err := svc.Sessions.SetMode(v.ID, mode); err != nil {
			t.Fatal(err)
		}
		if err := svc.Sessions.Send(ctx, v.ID, q); err != nil {
			t.Fatal(err)
		}
		var asked []string
		deadline := time.Now().Add(4 * time.Minute)
		for time.Now().Before(deadline) {
			cur := view(svc, v.ID)
			if !cur.Busy {
				return asked, last(t, svc, v.ID)
			}
			if len(cur.Approvals) > 0 {
				a := cur.Approvals[0]
				asked = append(asked, strings.TrimPrefix(a.Tool, "mcp__notes__"))
				d := session.Allow
				if strings.HasSuffix(a.Tool, "delete_note") {
					d = session.Deny
				}
				svc.Sessions.Answer(v.ID, a.RequestID, d, "")
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Fatalf("%s turn never finished", mode)
		return nil, store.Page{}
	}
	ran := func(p store.Page) map[string]string {
		out := map[string]string{}
		for _, it := range p.Trace {
			name := strings.TrimPrefix(it.Name, "mcp__notes__")
			switch {
			case it.Denied:
				out[name] = "refused"
			case it.OK:
				out[name] = "ran"
			}
		}
		return out
	}
	check := func(mode string, asked []string, p store.Page, wantAsked string, want map[string]string) {
		t.Helper()
		got := ran(p)
		for tool, w := range want {
			if got[tool] != w {
				t.Errorf("%s: %s %s, want %s (trace %+v)", mode, tool, got[tool], w, p.Trace)
			}
		}
		if strings.Join(asked, ",") != wantAsked {
			t.Errorf("%s: asked about %v, want %s", mode, asked, wantAsked)
		}
	}

	asked, p := run(session.ModeAlways)
	check("always", asked, p, "touch_note,delete_note", map[string]string{"read_note": "ran", "touch_note": "ran", "delete_note": "refused"})
	asked, p = run(session.ModeNever)
	check("never", asked, p, "", map[string]string{"read_note": "ran", "touch_note": "ran", "delete_note": "ran"})

	// Prompt when unsafe, after marking touch_note safe for this project:
	// only the destructive tool prompts.
	if err := svc.Sessions.Teach(v.ID, []store.SafeClass{{Kind: store.KindTool, Words: "mcp__notes__touch_note"}}, store.Safe, session.ScopeProject); err != nil {
		t.Fatal(err)
	}
	asked, p = run(session.ModeUnsafe)
	check("unsafe, taught", asked, p, "delete_note", map[string]string{"read_note": "ran", "touch_note": "ran", "delete_note": "refused"})

	// Always prompt me, unattended: reading runs, the rest is declined.
	svc.Sessions.SetUnattended(v.ID, true)
	asked, p = run(session.ModeAlways)
	check("always, unattended", asked, p, "", map[string]string{"read_note": "ran", "touch_note": "refused", "delete_note": "refused"})
}

// askUnknown sets Ask mode to ask about commands and tools UNCLI doesn't
// recognise, instead of having the quick-task model judge them.
func askUnknown(t *testing.T, svc *app.Service) {
	t.Helper()
	p := svc.Preferences()
	p.UnknownCommands = session.UnknownAsk
	if _, err := svc.SetPreferences(p); err != nil {
		t.Fatal(err)
	}
}

// The quick-task model judges commands UNCLI doesn't recognise: harmless
// ones as looks or routine, harmful ones as risky with a reason.
func TestModelJudgesCommands(t *testing.T) {
	svc, _ := open(t, t.TempDir())
	defer svc.Close()
	if _, err := svc.InstallCLI(context.Background()); err != nil {
		t.Fatal(err)
	}
	v, err := svc.Sessions.Create("code", t.TempDir(), "haiku")
	if err != nil {
		t.Fatal(err)
	}
	for c, want := range map[string]string{
		`node -e "console.log(process.version)"`:                          "allow",
		`node -e "require('child_process').execSync('git push --force')"`: "ask",
		`python -c "import shutil; shutil.rmtree('C:/Users')"`:            "ask",
	} {
		e, err := svc.Sessions.ExplainCommand(v.ID, c)
		if err != nil {
			t.Fatal(err)
		}
		if e.Action != want || len(e.Why) == 0 || e.Why[0].Judged == "" || e.Why[0].Note == "" {
			t.Errorf("%s: %+v", c, e)
		}
		t.Logf("%s -> %s %+v", c, e.Action, e.Why)
	}
}

//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"uncli/internal/adapter/antigravity"
	"uncli/internal/app"
	"uncli/internal/session"
)

// The Antigravity CLI through UNCLI (decision 0012): the pinned download,
// a session whose tool calls are approved on UNCLI's cards through its
// hook, resume after a restart, and the CLI refusing a tool call whenever
// the hook fails. It uses the machine's Google sign-in for Antigravity.

// hookCommand builds UNCLI's hook mode on its own and makes the app use it
// (the app itself is the hook when built).
func hookCommand(t *testing.T) {
	t.Helper()
	// A built UNCLI, to check the app itself as the hook.
	if exe := os.Getenv("UNCLI_APP_EXE"); exe != "" {
		t.Setenv(app.HookCommandEnv, `"`+exe+`" hook`)
		return
	}
	bin := filepath.Join(t.TempDir(), "uncli-hook")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	out, err := exec.Command("go", "build", "-o", bin, "../../cmd/uncli-hook").CombinedOutput()
	if err != nil {
		t.Fatalf("building the hook: %v\n%s", err, out)
	}
	t.Setenv(app.HookCommandEnv, `"`+bin+`"`)
}

// installAgy downloads the pinned CLI (shared with the app's cache) and
// skips the test when it isn't signed in.
func installAgy(t *testing.T, svc *app.Service) {
	t.Helper()
	st, err := svc.InstallProvider(context.Background(), "antigravity")
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !st.Installed || st.Version != antigravity.PinnedVersion {
		t.Fatalf("status after install = %+v", st)
	}
	if !st.LoggedIn {
		t.Skip("the Antigravity CLI isn't signed in on this machine; sign in to Antigravity and rerun")
	}
	if len(st.Models) == 0 {
		t.Errorf("no models listed: %+v", st)
	}
}

// waitFor polls a session until ok, or fails.
func waitFor(t *testing.T, svc *app.Service, id string, ok func(session.View) bool, what string) session.View {
	t.Helper()
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		if v := view(svc, id); ok(v) {
			return v
		}
		time.Sleep(200 * time.Millisecond)
	}
	v := view(svc, id)
	pages, _ := svc.Sessions.Pages(id)
	var p any
	if len(pages) > 0 {
		p = pages[len(pages)-1]
	}
	t.Fatalf("waited in vain for %s: state %s, error %q\nlast page: %+v", what, v.State, v.Error, p)
	return session.View{}
}

func TestAntigravity(t *testing.T) {
	hookCommand(t)
	config := t.TempDir()
	svc, _ := open(t, config)
	installAgy(t, svc)
	t.Logf("Antigravity CLI %s installed and signed in", antigravity.PinnedVersion)

	work := t.TempDir()
	v, _, err := svc.CreateSessionWith(session.NewSession{Profile: "code", Workdir: work, Model: "gemini-3.8-flash-low", Provider: "antigravity"})
	if err != nil {
		t.Fatal(err)
	}
	if v.Adapter != "antigravity" {
		t.Fatalf("session = %+v", v)
	}
	if _, err := svc.Sessions.SetMode(v.ID, session.ModeAlways); err != nil {
		t.Fatal(err)
	}

	// A write waits on a card; allowed, it happens.
	go func() {
		_ = svc.Sessions.Send(context.Background(), v.ID, "Use your write_to_file tool to create hello.txt in the current folder containing exactly: hi from agy. Then reply DONE.")
	}()
	card := waitFor(t, svc, v.ID, func(v session.View) bool { return len(v.Approvals) > 0 }, "a card for the write").Approvals[0]
	if card.Tool != "write_to_file" || !strings.HasSuffix(card.Action.Path, "hello.txt") {
		t.Fatalf("card = %+v", card)
	}
	if err := svc.Sessions.Answer(v.ID, card.RequestID, session.Allow, ""); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, svc, v.ID)
	if b, err := os.ReadFile(filepath.Join(work, "hello.txt")); err != nil || !strings.Contains(string(b), "hi from agy") {
		t.Fatalf("hello.txt = %q, %v", b, err)
	}
	p := last(t, svc, v.ID)
	if p.Status != "done" || p.InputTokens == 0 || len(p.Trace) == 0 || p.Trace[0].Approved != "you" {
		t.Errorf("page = %+v", p)
	}
	if len(p.TouchedFiles) != 1 || !strings.HasSuffix(p.TouchedFiles[0].Path, "hello.txt") {
		t.Errorf("touched = %+v", p.TouchedFiles)
	}

	// A delete waits on a card; denied, it doesn't happen.
	go func() {
		_ = svc.Sessions.Send(context.Background(), v.ID, "Run this shell command with your command tool: Remove-Item hello.txt . Then reply DONE.")
	}()
	card = waitFor(t, svc, v.ID, func(v session.View) bool { return len(v.Approvals) > 0 }, "a card for the delete").Approvals[0]
	if card.Tool != "run_command" {
		t.Fatalf("card = %+v", card)
	}
	if err := svc.Sessions.Answer(v.ID, card.RequestID, session.Deny, ""); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, svc, v.ID)
	if _, err := os.Stat(filepath.Join(work, "hello.txt")); err != nil {
		t.Errorf("hello.txt was deleted after a denial: %v", err)
	}
	if p := last(t, svc, v.ID); len(p.Trace) == 0 || !p.Trace[len(p.Trace)-1].Denied {
		t.Errorf("trace = %+v", p.Trace)
	}

	// After a restart, the conversation resumes.
	svc.Close()
	svc, _ = open(t, config)
	defer svc.Close()
	p = send(t, svc, v.ID, "What is the name of the file you created earlier? Reply with just the file name.")
	if !strings.Contains(strings.ToLower(p.AnswerMD), "hello.txt") {
		t.Errorf("after a restart: %q", p.AnswerMD)
	}
}

// Every way UNCLI's hook can fail makes the CLI refuse the tool call, with
// its own checks off (decision 0012): the reason it's safe to switch them off.
func TestAntigravityHookFailsClosed(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the failing hooks are Windows scripts")
	}
	svc, _ := open(t, t.TempDir())
	defer svc.Close()
	installAgy(t, svc)
	bin, ok := antigravity.NewInstaller(filepath.Join(svc.Paths.Cache, "cli", "antigravity")).Path(antigravity.PinnedVersion)
	if !ok {
		t.Fatal("no pinned binary")
	}
	failures := map[string]struct {
		script  string
		timeout int
	}{
		"crash":     {"@exit /b 1", 30},
		"timeout":   {"@ping -n 20 127.0.0.1 >nul\r\n@echo {\"decision\":\"allow\"}", 3},
		"not json":  {"@echo not json", 30},
		"no answer": {"@echo {}", 30},
	}
	for name, f := range failures {
		t.Run(name, func(t *testing.T) {
			state, work := t.TempDir(), t.TempDir()
			cfg := filepath.Join(state, "config")
			_ = os.MkdirAll(cfg, 0o755)
			_ = os.WriteFile(filepath.Join(cfg, "hook.cmd"), []byte(f.script+"\r\n"), 0o755)
			hooks, _ := json.Marshal(map[string]any{"uncli": map[string]any{"PreToolUse": []any{map[string]any{
				"matcher": "*", "hooks": []any{map[string]any{"type": "command", "command": "hook.cmd", "timeout": f.timeout}}}}}})
			_ = os.WriteFile(filepath.Join(cfg, "hooks.json"), hooks, 0o644)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			cmd := exec.CommandContext(ctx, bin, "--gemini_dir="+state, "--dangerously-skip-permissions", "--model", "gemini-3.8-flash-low",
				"-p", "Use your write_to_file tool to create made.txt in the current folder containing: x. Then reply DONE.", "--output-format", "json")
			cmd.Dir = work
			cmd.Env = append(os.Environ(), "AGY_CLI_DISABLE_AUTO_UPDATE=true")
			out, _ := cmd.CombinedOutput()
			if _, err := os.Stat(filepath.Join(work, "made.txt")); err == nil {
				t.Errorf("the write ran past a failed hook; output: %s", out)
			}
			if !strings.Contains(string(out), "hook") {
				t.Logf("output: %s", out)
			}
		})
	}
}

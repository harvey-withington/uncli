package antigravity

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"uncli/internal/core"
)

func TestBuildCommand(t *testing.T) {
	dir := t.TempDir()
	a := New(nil, dir, `C:\Program Files\UNCLI\uncli.exe --hook`)
	cmd, err := a.BuildCommand("agy.exe", core.LaunchSpec{Approvals: true, ResumeID: "abc", Model: "gemini-3.8-flash-low",
		Env: map[string]string{"UNCLI_HOOK_TOKEN": "t"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--gemini_dir=" + dir, "--input-format", "stream-json", "--output-format", "stream-json",
		"--dangerously-skip-permissions", "--conversation", "abc", "--model", "gemini-3.8-flash-low"}
	if !slices.Equal(cmd.Args, want) {
		t.Errorf("args = %q", cmd.Args)
	}
	if cmd.Env["AGY_CLI_DISABLE_AUTO_UPDATE"] != "true" || cmd.Env["UNCLI_HOOK_TOKEN"] != "t" {
		t.Errorf("env = %v", cmd.Env)
	}

	// The hook is in place before the CLI's own checks are switched off.
	hooks, err := os.ReadFile(filepath.Join(dir, "config", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var h map[string]map[string]json.RawMessage
	if err := json.Unmarshal(hooks, &h); err != nil || h["uncli"]["PreToolUse"] == nil || h["uncli"]["PreInvocation"] == nil {
		t.Fatalf("hooks.json = %s (%v)", hooks, err)
	}
	var pre []struct {
		Matcher string `json:"matcher"`
		Hooks   []struct {
			Command string `json:"command"`
			Timeout int    `json:"timeout"`
		} `json:"hooks"`
	}
	_ = json.Unmarshal(h["uncli"]["PreToolUse"], &pre)
	if len(pre) != 1 || pre[0].Matcher != "*" || len(pre[0].Hooks) != 1 || pre[0].Hooks[0].Timeout != HookTimeout {
		t.Errorf("the tool hook doesn't match every tool: %s", h["uncli"]["PreToolUse"])
	}
	// The command line reaches the hook through the environment, so a path
	// with spaces survives the CLI's quoting (recorded 1.3.1).
	if cmd.Env[HookCmdEnv] != `C:\Program Files\UNCLI\uncli.exe --hook` || !strings.Contains(pre[0].Hooks[0].Command, HookCmdEnv) {
		t.Errorf("hook command = %q, env %q", pre[0].Hooks[0].Command, cmd.Env[HookCmdEnv])
	}

	// Without approvals the CLI keeps its own checks, which deny headless.
	cmd, _ = a.BuildCommand("agy.exe", core.LaunchSpec{})
	if slices.Contains(cmd.Args, "--dangerously-skip-permissions") {
		t.Errorf("args = %q", cmd.Args)
	}
	// Approvals without a hook to give them is refused.
	if _, err := New(nil, t.TempDir(), "").BuildCommand("agy.exe", core.LaunchSpec{Approvals: true}); err == nil {
		t.Error("approvals with no hook were allowed")
	}
}

func TestAnswerHook(t *testing.T) {
	a := &Adapter{}
	tool := core.HookCall{Kind: core.HookTool}
	if b, _ := a.AnswerHook(tool, core.HookAnswer{Allow: true}); string(b) != `{"decision":"allow"}` {
		t.Errorf("allow = %s", b)
	}
	if b, _ := a.AnswerHook(tool, core.HookAnswer{Reason: "no"}); string(b) != `{"decision":"deny","reason":"no"}` {
		t.Errorf("deny = %s", b)
	}
	ins := core.HookCall{Kind: core.HookInstructions}
	if b, _ := a.AnswerHook(ins, core.HookAnswer{Instructions: "Be brief."}); string(b) != `{"injectSteps":[{"ephemeralMessage":"Be brief."}]}` {
		t.Errorf("instructions = %s", b)
	}
	if b, _ := a.AnswerHook(ins, core.HookAnswer{}); string(b) != `{}` {
		t.Errorf("no instructions = %s", b)
	}
	if _, err := a.ReadHook(EventPreTool, []byte(`{"toolCall":{}}`)); err == nil {
		t.Error("a call naming no tool was read")
	}
}

func TestParseModels(t *testing.T) {
	out := "Fetching available models...\ngemini-3.8-flash-high\tGemini 3.8 Flash (High)\r\nclaude-opus-5-5-low\tClaude Opus 5.5 (Low)\n"
	ms := ParseModels([]byte(out))
	if len(ms) != 2 || ms[0].Value != "gemini-3.8-flash-high" || ms[1].DisplayName != "Claude Opus 5.5 (Low)" {
		t.Errorf("models = %+v", ms)
	}
	if info, _ := (&Adapter{}).ParseAuthStatus([]byte("error: not signed in")); info.LoggedIn {
		t.Error("signed in without models")
	}
}

func TestEncodeTurn(t *testing.T) {
	b, err := (&Adapter{}).EncodeTurn(core.UserTurn{Text: "hi", Directives: "Be brief."})
	if err != nil || string(b) != `{"event":"user","message":{"content":"Be brief.\n\nhi"}}`+"\n" {
		t.Errorf("turn = %s (%v)", b, err)
	}
	if _, err := (&Adapter{}).EncodeTurn(core.UserTurn{Text: "x", Attachments: []core.Attachment{{Name: "a.png"}}}); err == nil {
		t.Error("an attachment was encoded")
	}
}

func TestInstaller(t *testing.T) {
	bin := []byte("agy binary")
	sum := sha512.Sum512(bin)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manifests/windows_amd64.json":
			_ = json.NewEncoder(w).Encode(map[string]string{"version": "1.4.0", "url": "https://" + r.Host + "/bin", "sha512": hex.EncodeToString(sum[:])})
		case "/bin":
			_, _ = w.Write(bin)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	i := NewInstaller(t.TempDir())
	i.ManifestURL, i.Platform, i.Client = srv.URL+"/manifests", "windows_amd64", srv.Client()

	ch, err := i.Channels(context.Background())
	if err != nil || ch["latest"] != "1.4.0" {
		t.Fatalf("channels = %v, %v", ch, err)
	}
	// Neither pinned nor the latest: nowhere to get it from.
	if _, err := i.Ensure(context.Background(), "1.0.0", nil); err == nil {
		t.Error("an unknown version was downloaded")
	}
	// The manifest's URL is https; the test server's isn't reachable that way,
	// so point the build at it and check the checksum is enforced.
	pinned["9.9.9"] = map[string]Build{"windows_amd64": {URL: srv.URL + "/bin", SHA512: hex.EncodeToString(sum[:])}}
	pinned["9.9.8"] = map[string]Build{"windows_amd64": {URL: srv.URL + "/bin", SHA512: strings.Repeat("0", 128)}}
	defer delete(pinned, "9.9.9")
	defer delete(pinned, "9.9.8")
	p, err := i.Ensure(context.Background(), "9.9.9", nil)
	if err != nil || filepath.Base(p) != "agy.exe" {
		t.Fatalf("ensure = %s, %v", p, err)
	}
	if got, _ := os.ReadFile(p); string(got) != string(bin) {
		t.Errorf("binary = %q", got)
	}
	if _, err := i.Ensure(context.Background(), "9.9.8", nil); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Errorf("a bad checksum: %v", err)
	}
	if v, _ := i.Installed(); !slices.Equal(v, []string{"9.9.9"}) {
		t.Errorf("installed = %v", v)
	}
}

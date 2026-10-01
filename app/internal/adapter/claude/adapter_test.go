package claude

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"uncli/internal/core"
)

func TestEncodeTurnMatchesRecordedInput(t *testing.T) {
	a := New(nil)
	got, err := a.EncodeTurn(core.UserTurn{Text: "Say hello."})
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join(fixtures, "system-prompt-snapshot-resume.in.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var g, w map[string]any
	_ = json.Unmarshal(got, &g)
	_ = json.Unmarshal(want, &w)
	if fmt.Sprint(g) != fmt.Sprint(w) {
		t.Errorf("turn = %s\nrecorded = %s", got, want)
	}
	if !bytes.HasSuffix(got, []byte("\n")) {
		t.Error("turn must be one newline-terminated line")
	}
}

func TestEncodeTurnPutsDirectivesFirst(t *testing.T) {
	b, _ := New(nil).EncodeTurn(core.UserTurn{Text: "Q", Directives: "<session_directives>\nBe brief.\n</session_directives>"})
	var m struct {
		Message struct{ Content string } `json:"message"`
	}
	_ = json.Unmarshal(b, &m)
	if m.Message.Content != "<session_directives>\nBe brief.\n</session_directives>\n\nQ" {
		t.Errorf("content = %q", m.Message.Content)
	}
}

func TestEncodeControl(t *testing.T) {
	a := New(nil)
	cases := map[core.ControlKind]string{
		core.CtlInterrupt:  `"subtype":"interrupt"`,
		core.CtlSetModel:   `"model":"sonnet","subtype":"set_model"`,
		core.CtlInitialize: `"subtype":"initialize"`,
	}
	for kind, want := range cases {
		b, ok := a.EncodeControl(core.Control{Kind: kind, Model: "sonnet"})
		if !ok || !strings.Contains(string(b), want) || !strings.Contains(string(b), `"type":"control_request"`) {
			t.Errorf("%s -> %s", kind, b)
		}
	}
	b, ok := a.EncodeControl(core.Control{Kind: core.CtlApprove, RequestID: "r1", Allow: true, UpdatedInput: json.RawMessage(`{"a":1}`)})
	if !ok || !strings.Contains(string(b), `"behavior":"allow"`) || !strings.Contains(string(b), `"request_id":"r1"`) {
		t.Errorf("approve -> %s", b)
	}
	if _, ok := a.EncodeControl(core.Control{Kind: "nope"}); ok {
		t.Error("unknown control should be unsupported")
	}
}

func TestBuildCommand(t *testing.T) {
	a := New(nil)
	cmd, err := a.BuildCommand("claude.exe", core.LaunchSpec{
		ResumeID: "abc", SessionID: "ignored-when-resuming", Model: "haiku", Effort: "high",
		SystemPrompt: "sys", Tools: []string{"WebSearch", "Write"},
		AllowedTools:   []string{"Write(./artifacts/**)", "Bash(git status:*)"},
		PermissionMode: "acceptEdits", Isolated: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Join(cmd.Args, "|")
	for _, want := range []string{
		"-p|--input-format|stream-json|--output-format|stream-json|--verbose|--include-partial-messages",
		"--resume|abc", "--model|haiku", "--effort|high", "--system-prompt|sys", "--tools|WebSearch,Write",
		"--allowedTools|Write(./artifacts/**)|Bash(git status:*)|--permission-mode|acceptEdits",
		"--permission-prompts|none", "--strict-mcp-config|--setting-sources||--disable-slash-commands",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("args missing %q:\n%s", want, args)
		}
	}
	if strings.Contains(args, "--session-id") {
		t.Error("resume must not also pass --session-id")
	}
	if cmd.Env["DISABLE_AUTOUPDATER"] != "1" || !slices.Contains(cmd.EnvDrop, "CLAUDE_CODE_") {
		t.Errorf("env = %v drop = %v", cmd.Env, cmd.EnvDrop)
	}
	if _, err := a.BuildCommand("x", core.LaunchSpec{PermissionMode: "bypassPermissions"}); err == nil {
		t.Error("bypass mode must be refused")
	}
	cmd, _ = a.BuildCommand("x", core.LaunchSpec{SessionID: "new-id", Approvals: true})
	args = strings.Join(cmd.Args, "|")
	if !strings.Contains(args, "--session-id|new-id") || !strings.Contains(args, "--permission-prompt-tool|stdio") {
		t.Errorf("new session args = %s", args)
	}
}

func TestParseAuthStatus(t *testing.T) {
	info, err := New(nil).ParseAuthStatus([]byte(`{"loggedIn":true,"authMethod":"claude.ai","email":"a@b.c","subscriptionType":"max"}`))
	if err != nil || !info.LoggedIn || info.Subscription != "max" {
		t.Errorf("%+v %v", info, err)
	}
}

func fakeReleases(t *testing.T, payload []byte, checksum string) *httptest.Server {
	t.Helper()
	sum := sha256.Sum256(payload)
	if checksum == "" {
		checksum = hex.EncodeToString(sum[:])
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/stable", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "9.9.9\n") })
	mux.HandleFunc("/latest", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "9.9.10") })
	mux.HandleFunc("/9.9.9/manifest.json", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `{"version":"9.9.9","platforms":{"test-x64":{"binary":"claude","checksum":%q,"size":%d}}}`, checksum, len(payload))
	})
	mux.HandleFunc("/9.9.9/test-x64/claude", func(w http.ResponseWriter, _ *http.Request) { w.Write(payload) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestInstallerDownloadsAndVerifies(t *testing.T) {
	payload := bytes.Repeat([]byte("binary"), 50000)
	srv := fakeReleases(t, payload, "")
	inst := NewInstaller(t.TempDir())
	inst.BaseURL, inst.Platform = srv.URL, "test-x64"

	ch, err := inst.Channels(context.Background())
	if err != nil || ch["stable"] != "9.9.9" || ch["latest"] != "9.9.10" {
		t.Fatalf("channels = %v, %v", ch, err)
	}
	var last int64
	path, err := inst.Ensure(context.Background(), "9.9.9", func(done, total int64) { last = done })
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, payload) {
		t.Error("installed binary differs from download")
	}
	if last != int64(len(payload)) {
		t.Errorf("progress ended at %d of %d", last, len(payload))
	}
	if v, _ := inst.Installed(); !slices.Equal(v, []string{"9.9.9"}) {
		t.Errorf("installed = %v", v)
	}
	if p, ok := inst.Path("9.9.9"); !ok || p != path {
		t.Error("Path should find the installed binary")
	}
}

func TestInstallerRejectsBadChecksum(t *testing.T) {
	srv := fakeReleases(t, []byte("tampered"), strings.Repeat("0", 64))
	inst := NewInstaller(t.TempDir())
	inst.BaseURL, inst.Platform = srv.URL, "test-x64"
	if _, err := inst.Ensure(context.Background(), "9.9.9", nil); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("err = %v", err)
	}
	if _, ok := inst.Path("9.9.9"); ok {
		t.Error("a binary that failed verification must not be installed")
	}
	if _, err := inst.Ensure(context.Background(), "../evil", nil); err == nil {
		t.Error("path-like versions must be rejected")
	}
}

func TestDetectPlatform(t *testing.T) {
	for in, want := range map[[2]string]string{{"windows", "amd64"}: "win32-x64", {"darwin", "arm64"}: "darwin-arm64", {"linux", "amd64"}: "linux-x64", {"plan9", "386"}: ""} {
		if got := DetectPlatform(in[0], in[1]); got != want {
			t.Errorf("%v -> %q", in, got)
		}
	}
}

func TestLoginURL(t *testing.T) {
	// What "claude auth login" printed without a terminal (2.1.285).
	out := "Opening browser to sign in…\nIf the browser didn't open, visit: https://claude.com/cai/oauth/authorize?code=true&client_id=abc&response_type=code&state=xyz\nPaste code here if prompted > "
	u, ok := New(nil).LoginURL([]byte(out))
	if !ok || u != "https://claude.com/cai/oauth/authorize?code=true&client_id=abc&response_type=code&state=xyz" {
		t.Errorf("url = %q, %v", u, ok)
	}
	if _, ok := New(nil).LoginURL([]byte("Opening browser to sign in…\n")); ok {
		t.Error("no link yet")
	}
}

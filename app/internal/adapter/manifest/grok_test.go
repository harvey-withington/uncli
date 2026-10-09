package manifest

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"uncli/internal/core"
)

const grokManifest = "../../../testdata/providers/grok"

func grokAdapter(t *testing.T) *Adapter {
	t.Helper()
	m, _, err := Load(grokManifest)
	if err != nil {
		t.Fatal(err)
	}
	return New(m, nil, t.TempDir(), "")
}

func TestGrokManifest(t *testing.T) {
	a := grokAdapter(t)
	cmd, err := a.BuildCommand("grok.exe", core.LaunchSpec{Approvals: true, Model: "grok-4.6", Effort: "high", ResumeID: "s1"})
	if err != nil {
		t.Fatal(err)
	}
	// Options before the subcommand, nothing that approves by itself.
	want := []string{"agent", "--no-leader", "--model", "grok-4.6", "--reasoning-effort", "high", "stdio"}
	if !slices.Equal(cmd.Args, want) {
		t.Errorf("args = %q", cmd.Args)
	}
	if cmd.Env["GROK_HOME"] != a.StateDir {
		t.Errorf("env = %v", cmd.Env)
	}
	b, err := os.ReadFile(filepath.Join(a.StateDir, "config.toml"))
	if err != nil || !strings.Contains(string(b), "auto_update = false") {
		t.Errorf("config.toml = %q, %v", b, err)
	}
	c := a.Capabilities()
	if !c.Approvals || c.HookApprovals || !c.Interrupt || !c.Resume || c.LiveModelSwitch {
		t.Errorf("capabilities = %+v", c)
	}
	if a.NewDriver(core.LaunchSpec{}) == nil {
		t.Error("no ACP driver")
	}

	// Signed out, as recorded (grok 1.0.50).
	out := "You are not authenticated.\n\nDefault model: grok-4.6\n\nAvailable models:\n"
	if st, _ := a.ParseAuthStatus([]byte(out)); st.LoggedIn {
		t.Error("signed out read as signed in")
	}
	if st, _ := a.ParseAuthStatus([]byte("Default model: grok-4.6\n\nAvailable models:\n  grok-4.6\n")); !st.LoggedIn {
		t.Error("signed in read as signed out")
	}
	login := a.LoginCommand("grok.exe")
	if !slices.Equal(login.Args, []string{"login", "--device-auth"}) || login.Env["GROK_HOME"] != a.StateDir {
		t.Errorf("login = %+v", login)
	}
}

// ACP plugins have ACP's approvals, and nothing else.
func TestACPManifestChecks(t *testing.T) {
	b, _ := os.ReadFile(filepath.Join(grokManifest, File))
	src := string(b)
	for name, c := range map[string]struct{ src, want string }{
		"approves by itself":  {strings.Replace(src, `tail: ["stdio"]`, "tail: [\"stdio\"]\n  approvals: [\"--always-approve\"]", 1), "approved through ACP"},
		"event rules":         {src + "events:\n  - when: {a: b}\n", "no events or turn"},
		"no sign-in command":  {strings.Replace(src, `  login: ["login", "--device-auth"]`, "", 1), "needs status.login"},
		"a file outside":      {strings.Replace(src, `"{state}/config.toml"`, `"{state}/../x.toml"`, 1), "only files in {state}"},
		"an unknown protocol": {strings.Replace(src, "protocol: acp", "protocol: grpc", 1), "lines or acp"},
	} {
		if _, err := Parse([]byte(c.src)); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

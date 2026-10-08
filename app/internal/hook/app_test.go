package hook

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

// The built app is the hook (main.go): a Windows GUI program, run by the
// CLI through cmd /c with its command line in an environment variable
// (decision 0012). It must still read the call on stdin and print the
// answer. Point UNCLI_APP_EXE at a built UNCLI to check.
func TestBuiltAppAsHook(t *testing.T) {
	exe := os.Getenv("UNCLI_APP_EXE")
	if exe == "" {
		t.Skip("set UNCLI_APP_EXE to a built UNCLI")
	}
	s, err := Listen()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var got string
	env, _ := s.Register(func(_ context.Context, event string, payload []byte) ([]byte, error) {
		got = event + " " + string(payload)
		return []byte(`{"decision":"allow"}`), nil
	})
	cmdline := `"` + exe + `" hook`
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "%UNCLI_HOOK_CMD% pre-tool")
	} else {
		cmd = exec.Command("sh", "-c", `eval "$UNCLI_HOOK_CMD pre-tool"`)
	}
	cmd.Env = append(os.Environ(), "UNCLI_HOOK_CMD="+cmdline, AddrEnv+"="+env[AddrEnv], TokenEnv+"="+env[TokenEnv])
	cmd.Stdin = strings.NewReader(`{"toolCall":{"name":"view_file"}}`)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if strings.TrimSpace(string(out)) != `{"decision":"allow"}` || got != `pre-tool {"toolCall":{"name":"view_file"}}` {
		t.Errorf("printed %q; UNCLI saw %q", out, got)
	}
}

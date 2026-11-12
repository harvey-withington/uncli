package main

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// An args.txt line splits as the CLI got it: a prompt with spaces is one
// argument, "" and a valued flag with nothing after it are empty ones.
func TestSplitArgs(t *testing.T) {
	got := splitArgs(`-p --verbose --system-prompt Always start with AHOY. --allowedTools Bash(git status:*) --tools "" --setting-sources  --disable-slash-commands`)
	want := []string{"-p", "--verbose", "--system-prompt", "Always start with AHOY.", "--allowedTools", "Bash(git status:*)", "--tools", "", "--setting-sources", "", "--disable-slash-commands"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

// A recorded answer goes to the request the CLI actually made, allowing
// the input it actually asked about.
func TestAnswerFollowsTheRequest(t *testing.T) {
	rec := `{"type":"control_response","response":{"subtype":"success","request_id":"old","response":{"behavior":"allow","updatedInput":{"file_path":"C:\\old\\hello.txt"}}}}`
	req := parse(`{"type":"control_request","request_id":"new-id","request":{"subtype":"can_use_tool","input":{"file_path":"C:\\new\\hello.txt","content":"hi"}}}`)
	var a struct {
		Response struct {
			RequestID string `json:"request_id"`
			Response  struct {
				UpdatedInput map[string]string `json:"updatedInput"`
			} `json:"response"`
		} `json:"response"`
	}
	if err := json.Unmarshal([]byte(answer(rec, req)), &a); err != nil {
		t.Fatal(err)
	}
	if a.Response.RequestID != "new-id" || a.Response.Response.UpdatedInput["file_path"] != `C:\new\hello.txt` {
		t.Errorf("answer = %+v", a)
	}
	if !strings.Contains(answer("", req), `"behavior":"deny"`) {
		t.Error("a request the recording didn't answer is denied")
	}
}

// A path is scrubbed as it is, JSON-escaped, and escaped twice.
func TestReplaceEscaped(t *testing.T) {
	in := `C:\Users\me\w {"p":"C:\\Users\\me\\w"} {\"p\":\"C:\\\\Users\\\\me\\\\w\"}`
	want := `C:\work {"p":"C:\\work"} {\"p\":\"C:\\\\work\"}`
	if got := replaceEscaped(in, `C:\Users\me\w`, `C:\work`); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// A recording runs in its own config folder on the models-only token:
// nothing of the user's sign-in, config or CLI session comes with it.
func TestCLIEnvIsClean(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", `C:\Users\me\.claude`)
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "mine")
	t.Setenv("ANTHROPIC_API_KEY", "mine")
	t.Setenv("CLAUDECODE", "1")
	t.Setenv("CLAUDE_CODE_ENTRYPOINT", "cli")
	env := strings.Join(cliEnv(&fixture{name: "single-turn", config: `C:\tmp\cfg`, token: "models-only"}), "\n")
	for _, bad := range []string{"mine", `C:\Users\me`, "CLAUDECODE", "ENTRYPOINT"} {
		if strings.Contains(env, bad) {
			t.Errorf("%s came through", bad)
		}
	}
	if !strings.Contains(env, `CLAUDE_CONFIG_DIR=C:\tmp\cfg`) || !strings.Contains(env, "CLAUDE_CODE_OAUTH_TOKEN=models-only") {
		t.Errorf("env = %s", env)
	}
	signedOut := strings.Join(cliEnv(&fixture{name: "error-not-logged-in", config: `C:\tmp\cfg`, token: "models-only"}), "\n")
	if strings.Contains(signedOut, "OAUTH_TOKEN") {
		t.Error("the signed-out fixture got a token")
	}
}

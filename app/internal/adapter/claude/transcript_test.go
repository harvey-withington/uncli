package claude

import (
	"os"
	"strings"
	"testing"

	"uncli/internal/core"
)

var transcripts = core.TranscriptLocation{ConfigDir: "../../../testdata/transcripts/claude/" + PinnedVersion}

const fixtureID = "939053cc-a279-4983-bad0-d37f5fab3c0d"

func TestListTranscripts(t *testing.T) {
	list, err := New(nil).ListTranscripts(transcripts)
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %+v %v", list, err)
	}
	i := list[0]
	if i.ID != fixtureID || i.Workdir != `C:\uncli-spike\w-import` || i.Turns != 4 || i.CLIVersion != "2.1.285" ||
		!strings.HasPrefix(i.FirstQuestion, "Use the Write tool to create hello.txt") || i.Started == 0 || i.Updated < i.Started {
		t.Errorf("info = %+v", i)
	}
}

func TestReadTranscript(t *testing.T) {
	tr, err := New(nil).ReadTranscript(transcripts, fixtureID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tr.Turns) != 4 {
		t.Fatalf("turns = %d: %+v", len(tr.Turns), tr.Turns)
	}
	first := tr.Turns[0]
	if !strings.Contains(first.Answer, "DONE") || first.Model != "claude-haiku-4-5-20251001" || first.Usage.OutputTokens == 0 {
		t.Errorf("first turn = %+v", first)
	}
	if len(first.Tools) != 1 || first.Tools[0].Name != "Write" || !first.Tools[0].Done || !first.Tools[0].OK || first.Tools[0].Summary != "Write hello.txt" {
		t.Errorf("tools = %+v", first.Tools)
	}
	if len(first.Files) != 1 || !strings.HasSuffix(first.Files[0].Path, "hello.txt") || first.Files[0].How != "write" || first.Files[0].Added != 1 {
		t.Errorf("files = %+v", first.Files)
	}
	if second := tr.Turns[1]; !strings.Contains(second.Answer, "hello.txt") || len(second.Tools) != 0 {
		t.Errorf("second = %+v", second)
	}
	// The slash command is a page of its own; the compaction summary and
	// its meta lines aren't pages, nor part of an answer.
	if c := tr.Turns[2]; !c.Command || c.Question != "/compact" || c.Answer != "Compacted" {
		t.Errorf("command = %+v", c)
	}
	if last := tr.Turns[3]; last.Question != "Reply with just the word AFTER." || last.Answer != "AFTER" || last.FinishedAt < last.StartedAt {
		t.Errorf("last = %+v", last)
	}
	for _, turn := range tr.Turns {
		if strings.Contains(turn.Answer, "continued from a previous conversation") {
			t.Error("the compaction summary leaked into an answer")
		}
	}
	if _, err := New(nil).ReadTranscript(transcripts, "../../etc"); err == nil {
		t.Error("read a path that isn't an id")
	}
}

// Lines the reader doesn't know, or can't parse, are skipped, never an error.
func TestTranscriptSkipsUnknownLines(t *testing.T) {
	r := strings.NewReader("{not json\n{\"type\":\"brand-new-kind\"}\n" +
		`{"type":"user","timestamp":"2026-10-07T10:00:00Z","cwd":"/w","message":{"role":"user","content":"hi"}}` + "\n")
	var n int
	if err := eachLine(r, func(l tline) {
		if _, ok := question(l); ok {
			n++
		}
	}); err != nil || n != 1 {
		t.Errorf("questions = %d, %v", n, err)
	}
}

// TestReadRealTranscripts reads every transcript the CLI has saved on this
// computer and reports counts only (dev only: UNCLI_REAL_TRANSCRIPTS=1). A
// transcript that fails to read, or has turns with neither an answer nor a
// tool, is worth a look.
func TestReadRealTranscripts(t *testing.T) {
	if os.Getenv("UNCLI_REAL_TRANSCRIPTS") != "1" {
		t.Skip("set UNCLI_REAL_TRANSCRIPTS=1 to read this computer's transcripts")
	}
	home, _ := os.UserHomeDir()
	loc := core.TranscriptLocation{Home: home, ConfigDir: os.Getenv("CLAUDE_CONFIG_DIR")}
	a := New(nil)
	list, err := a.ListTranscripts(loc)
	if err != nil {
		t.Fatal(err)
	}
	var turns, empty, tools, files, commands int
	for _, info := range list {
		tr, err := a.ReadTranscript(loc, info.ID)
		if err != nil {
			t.Errorf("%s: %v", info.ID, err)
			continue
		}
		for _, turn := range tr.Turns {
			turns++
			tools += len(turn.Tools)
			files += len(turn.Files)
			if turn.Command {
				commands++
			} else if turn.Answer == "" && len(turn.Tools) == 0 {
				empty++
			}
		}
	}
	t.Logf("%d transcripts, %d turns (%d slash commands), %d tool uses, %d files written, %d turns with nothing in them",
		len(list), turns, commands, tools, files, empty)
}

// The CLI saves its note to itself that a background task finished as a
// user line; it becomes a turn the CLI started, with no question.
func TestTranscriptTaskNotificationIsACLITurn(t *testing.T) {
	cfg := t.TempDir()
	dir := cfg + "/projects/x"
	_ = os.MkdirAll(dir, 0o755)
	lines := `{"type":"user","timestamp":"2026-10-07T10:00:00Z","cwd":"/w","message":{"role":"user","content":"Start a background agent."}}
{"type":"assistant","timestamp":"2026-10-07T10:00:05Z","message":{"id":"m1","model":"claude-haiku-4-5-20251001","content":[{"type":"text","text":"Launched it."}]}}
{"type":"user","timestamp":"2026-10-07T10:00:09Z","origin":{"kind":"task-notification"},"message":{"role":"user","content":"<task-notification>\n<task-id>a1</task-id>\n<status>completed</status>\n<summary>Agent \"x\" finished</summary>\n</task-notification>"}}
{"type":"assistant","timestamp":"2026-10-07T10:00:12Z","message":{"id":"m2","model":"claude-haiku-4-5-20251001","content":[{"type":"text","text":"It replied PING."}]}}
`
	id := "aaaaaaaa-1111-2222-3333-444444444444"
	if err := os.WriteFile(dir+"/"+id+".jsonl", []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
	loc := core.TranscriptLocation{ConfigDir: cfg}
	tr, err := New(nil).ReadTranscript(loc, id)
	if err != nil || len(tr.Turns) != 2 {
		t.Fatalf("turns = %+v, %v", tr.Turns, err)
	}
	if c := tr.Turns[1]; c.Origin != "cli" || c.Question != "" || c.Answer != "It replied PING." {
		t.Errorf("CLI turn = %+v", c)
	}
	list, _ := New(nil).ListTranscripts(loc)
	if len(list) != 1 || list[0].FirstQuestion != "Start a background agent." {
		t.Errorf("list = %+v", list)
	}
}

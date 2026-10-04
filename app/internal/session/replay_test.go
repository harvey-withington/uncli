package session

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"uncli/config"
	"uncli/internal/profile"
)

// TestReplayPrompts measures how often each prompt level would prompt on
// real tool use, and how much teaching ("This is safe", once per class)
// cuts that. It reads, read-only and locally, either a UNCLI database
// (UNCLI_REPLAY_DB: its approval_asked and tool_started events) or a
// list of folders of Claude Code transcripts (UNCLI_REPLAY_TRANSCRIPTS, split
// like PATH: every
// tool_use in *.jsonl), and logs counts and the classes that prompt most.
// Nothing it reads is kept. Skipped unless one of the variables is set.
//
//	UNCLI_REPLAY_TRANSCRIPTS=%USERPROFILE%\.claude\projects go test -run TestReplayPrompts -v ./internal/session/
func TestReplayPrompts(t *testing.T) {
	uses := replayUses(t)
	if len(uses) == 0 {
		t.Skip("set UNCLI_REPLAY_DB or UNCLI_REPLAY_TRANSCRIPTS to replay real tool use")
	}
	defaults, _ := fs.Sub(config.Defaults, "defaults")
	set, err := profile.Load(defaults, "")
	if err != nil {
		t.Fatal(err)
	}
	code, _ := set.Profile("code")

	for _, unknownSetting := range []string{UnknownAsk, UnknownInside} {
		for _, mode := range []string{ModeAlways, ModeUnsafe, ModeNever} {
			prompts, unknowns := 0, 0
			reasons := map[string]int{}
			classes := map[string]int{}
			taught := map[string]bool{}
			afterTeaching := 0
			for _, u := range uses {
				p := policy{mode: mode, unknown: unknownSetting, workdir: u.workdir, cwd: u.cwd,
					profile: parseAllowlist(code.AllowedTools, u.workdir)}
				v := p.judgeTool(u.tool, u.input)
				if v.action != actionPrompt {
					continue
				}
				prompts++
				learn, ok := learnable(v.why)
				keys := []string{}
				for _, r := range v.why {
					switch r.By {
					case "always", "unsafe", "unknown", "user":
						reasons[r.By+" "+r.Risk+r.Fixed]++
						if r.By == "unknown" {
							unknowns++
						}
						if r.Class != nil {
							classes[r.Class.Words+" "+r.Class.Flags]++
						}
					}
				}
				for _, c := range learn {
					keys = append(keys, c.Kind+":"+c.Words+"|"+c.Flags)
				}
				// Simulated teaching: the first prompt for a class is answered
				// "This is safe"; later ones for the same classes don't prompt.
				seen := ok
				for _, k := range keys {
					seen = seen && taught[k]
				}
				if !seen {
					afterTeaching++
				}
				if ok {
					for _, k := range keys {
						taught[k] = true
					}
				}
			}
			t.Logf("%-6s unknown=%-6s: %d of %d tool uses prompt (%.1f%%; %d can't be placed); with teaching once per class: %d",
				mode, unknownSetting, prompts, len(uses), 100*float64(prompts)/float64(len(uses)), unknowns, afterTeaching)
			if mode == ModeUnsafe {
				t.Logf("    why: %s", top(reasons, 12))
				t.Logf("    classes: %s", top(classes, 25))
			}
		}
	}
}

type replayUse struct {
	tool    string
	input   json.RawMessage
	workdir string // the project: the first folder of its transcript
	cwd     string // where the shell was
}

func replayUses(t *testing.T) []replayUse {
	var out []replayUse
	if path := os.Getenv("UNCLI_REPLAY_DB"); path != "" {
		db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		rows, err := db.Query(`SELECT e.data, s.workdir FROM events e JOIN sessions s ON s.id = e.session_id WHERE e.kind = 'tool_started'`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var data, workdir string
			if rows.Scan(&data, &workdir) != nil {
				continue
			}
			var ts struct {
				Name  string          `json:"name"`
				Input json.RawMessage `json:"input"`
			}
			if json.Unmarshal([]byte(data), &ts) == nil && ts.Name != "" {
				out = append(out, replayUse{ts.Name, ts.Input, workdir, workdir})
			}
		}
	}
	for _, dir := range filepath.SplitList(os.Getenv("UNCLI_REPLAY_TRANSCRIPTS")) {
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".jsonl") {
				return nil
			}
			f, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer f.Close()
			root := ""
			sc := bufio.NewScanner(f)
			sc.Buffer(make([]byte, 1<<20), 64<<20)
			for sc.Scan() {
				var l struct {
					Type    string `json:"type"`
					Cwd     string `json:"cwd"`
					Message struct {
						Content json.RawMessage `json:"content"`
					} `json:"message"`
				}
				if json.Unmarshal(sc.Bytes(), &l) != nil || l.Type != "assistant" {
					continue
				}
				var blocks []struct {
					Type  string          `json:"type"`
					Name  string          `json:"name"`
					Input json.RawMessage `json:"input"`
				}
				_ = json.Unmarshal(l.Message.Content, &blocks)
				if root == "" {
					root = l.Cwd
				}
				for _, b := range blocks {
					if b.Type == "tool_use" && b.Name != "" {
						out = append(out, replayUse{b.Name, b.Input, root, l.Cwd})
					}
				}
			}
			return nil
		})
	}
	return out
}

func top(m map[string]int, n int) string {
	type kv struct {
		k string
		v int
	}
	var all []kv
	for k, v := range m {
		all = append(all, kv{k, v})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].v > all[j].v || all[i].v == all[j].v && all[i].k < all[j].k })
	var b strings.Builder
	for i, e := range all {
		if i == n {
			break
		}
		fmt.Fprintf(&b, "%s ×%d; ", strings.TrimSpace(e.k), e.v)
	}
	return b.String()
}

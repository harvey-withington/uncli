package claude

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"uncli/internal/core"
)

// Claude Code saves each conversation as JSON lines in
// <config>/projects/<encoded folder>/<session id>.jsonl, where <config> is
// CLAUDE_CONFIG_DIR or ~/.claude (fixture: testdata/transcripts/claude).
// The format is undocumented and changes between versions, so the reader
// takes only what it needs and skips anything else, as the stream parser
// does:
//
//   - a "user" line whose content is text starts a page (its question); one
//     whose content is tool results finishes those tools;
//   - an "assistant" line holds one content block: text joins the answer,
//     tool_use joins the trace; its message's model and usage (repeated on
//     each block of the message) are counted once per message id;
//   - a slash command arrives as "<command-name>…" text with its output as
//     "<local-command-stdout>…" text;
//   - a "<task-notification>…" user line is the CLI telling itself a
//     background task finished: a turn the CLI started, with no question;
//   - meta lines, compaction summaries, sidechain (sub-agent) lines and all
//     the bookkeeping types (attachment, queue-operation, …) are skipped.

// maxTranscript is the biggest transcript read; a longer one is cut there.
const maxTranscript = 64 << 20

type tline struct {
	Type             string          `json:"type"`
	UUID             string          `json:"uuid"`
	SessionID        string          `json:"sessionId"`
	Timestamp        string          `json:"timestamp"`
	Cwd              string          `json:"cwd"`
	Version          string          `json:"version"`
	IsSidechain      bool            `json:"isSidechain"`
	IsMeta           bool            `json:"isMeta"`
	IsCompactSummary bool            `json:"isCompactSummary"`
	IsApiErrorMsg    bool            `json:"isApiErrorMessage"`
	Message          json.RawMessage `json:"message"`
	ToolUseResult    json.RawMessage `json:"toolUseResult"`
}

type tmessage struct {
	ID      string          `json:"id"`
	Model   string          `json:"model"`
	Content json.RawMessage `json:"content"`
	Usage   *resultUsage    `json:"usage"`
}

// projectsDir is where the CLI keeps transcripts for a location.
func projectsDir(loc core.TranscriptLocation) string {
	dir := loc.ConfigDir
	if dir == "" {
		dir = filepath.Join(loc.Home, ".claude")
	}
	return filepath.Join(dir, "projects")
}

var idRE = regexp.MustCompile(`^[0-9a-fA-F-]{8,64}$`)

// ListTranscripts lists the saved conversations, newest first.
func (a *Adapter) ListTranscripts(loc core.TranscriptLocation) ([]core.TranscriptInfo, error) {
	files, err := filepath.Glob(filepath.Join(projectsDir(loc), "*", "*.jsonl"))
	if err != nil {
		return nil, err
	}
	out := []core.TranscriptInfo{}
	for _, f := range files {
		id := strings.TrimSuffix(filepath.Base(f), ".jsonl")
		if !idRE.MatchString(id) {
			continue
		}
		info, err := scanInfo(f, id)
		if err != nil || info.Turns == 0 {
			continue // unreadable, or nothing was asked in it
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Updated > out[j].Updated })
	return out, nil
}

// scanInfo reads a transcript just far enough to describe it.
func scanInfo(path, id string) (core.TranscriptInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return core.TranscriptInfo{}, err
	}
	defer f.Close()
	info := core.TranscriptInfo{ID: id}
	if st, err := f.Stat(); err == nil {
		info.Updated = st.ModTime().UnixMilli()
	}
	err = eachLine(f, func(l tline) {
		if l.Cwd != "" && info.Workdir == "" {
			info.Workdir = l.Cwd
		}
		if l.Version != "" {
			info.CLIVersion = l.Version
		}
		if ts := stamp(l.Timestamp); ts > 0 {
			if info.Started == 0 {
				info.Started = ts
			}
			info.Updated = max(info.Updated, ts)
		}
		if q, ok := question(l); ok {
			info.Turns++
			if info.FirstQuestion == "" && !isCommand(q) && !isNotification(q) {
				info.FirstQuestion = oneLine(q, 200)
			}
		}
	})
	return info, err
}

// ReadTranscript turns one saved conversation into turns.
func (a *Adapter) ReadTranscript(loc core.TranscriptLocation, id string) (core.Transcript, error) {
	if !idRE.MatchString(id) {
		return core.Transcript{}, errors.New("not a conversation id")
	}
	files, _ := filepath.Glob(filepath.Join(projectsDir(loc), "*", id+".jsonl"))
	if len(files) == 0 {
		return core.Transcript{}, errors.New("that conversation is no longer saved by the CLI")
	}
	f, err := os.Open(files[0])
	if err != nil {
		return core.Transcript{}, err
	}
	defer f.Close()
	t := core.Transcript{Info: core.TranscriptInfo{ID: id}, Turns: []core.TranscriptTurn{}}
	var cur *core.TranscriptTurn
	counted := map[string]bool{} // assistant messages whose usage is in
	tools := map[string]int{}    // tool use id -> index in cur.Tools
	touching := map[string]core.FileTouched{}
	finish := func() {
		if cur == nil {
			return
		}
		cur.Answer = strings.TrimSpace(cur.Answer)
		t.Turns = append(t.Turns, *cur)
		cur = nil
	}
	err = eachLine(f, func(l tline) {
		ts := stamp(l.Timestamp)
		if l.Cwd != "" && t.Info.Workdir == "" {
			t.Info.Workdir = l.Cwd
		}
		if l.Version != "" {
			t.Info.CLIVersion = l.Version
		}
		if ts > 0 && t.Info.Started == 0 {
			t.Info.Started = ts
		}
		t.Info.Updated = max(t.Info.Updated, ts)
		if q, ok := question(l); ok {
			finish()
			cur = &core.TranscriptTurn{Question: q, StartedAt: ts, FinishedAt: ts, Tools: []core.TranscriptTool{}, Files: []core.FileTouched{}}
			switch {
			case isCommand(q):
				cur.Command, cur.Question = true, commandLine(q)
			case isNotification(q):
				cur.Origin, cur.Question = "cli", ""
			}
			clear(tools)
			t.Info.Turns++
			if t.Info.FirstQuestion == "" && !cur.Command && cur.Origin == "" {
				t.Info.FirstQuestion = oneLine(q, 200)
			}
			return
		}
		if cur == nil || l.IsSidechain || l.IsMeta || l.IsCompactSummary {
			return
		}
		if ts > 0 {
			cur.FinishedAt = ts
		}
		var m tmessage
		_ = json.Unmarshal(l.Message, &m)
		switch l.Type {
		case "assistant":
			if cur.Model == "" && m.Model != "" && m.Model != "<synthetic>" {
				cur.Model = m.Model
			}
			if m.Usage != nil && !counted[m.ID] {
				counted[m.ID] = true
				cur.Usage.InputTokens += m.Usage.InputTokens
				cur.Usage.OutputTokens += m.Usage.OutputTokens
				cur.Usage.CacheRead += m.Usage.CacheRead
				cur.Usage.CacheWrite += m.Usage.CacheWrite
			}
			if l.IsApiErrorMsg {
				cur.Error = true
			}
			for _, b := range blocks(m.Content) {
				switch b.Type {
				case "text":
					if cur.Answer != "" {
						cur.Answer += "\n\n"
					}
					cur.Answer += b.Text
				case "tool_use":
					tools[b.ID] = len(cur.Tools)
					cur.Tools = append(cur.Tools, core.TranscriptTool{ID: b.ID, Name: b.Name, Summary: toolSummary(b.Name, b.Input)})
					if ft, ok := fileTouched(b.Name, b.Input); ok {
						touching[b.ID] = ft
					}
				}
			}
		case "user":
			var s string
			if json.Unmarshal(m.Content, &s) == nil {
				if out, ok := commandOutput(s); ok && cur.Command {
					cur.Answer = out
				}
				return
			}
			bs := blocks(m.Content)
			for _, b := range bs {
				if b.Type != "tool_result" {
					continue
				}
				i, ok := tools[b.ToolUseID]
				if !ok {
					continue
				}
				tl := &cur.Tools[i]
				tl.Done, tl.OK, tl.Output = true, !b.IsError, truncate(resultText(b.Content))
				if ft, ok := touching[b.ToolUseID]; ok && !b.IsError {
					if len(bs) == 1 {
						ft.Line, ft.Added, ft.Removed = patchStats(l.ToolUseResult)
					}
					cur.Files = append(cur.Files, ft)
				}
				delete(touching, b.ToolUseID)
			}
		}
	})
	finish()
	t.Info.Turns = len(t.Turns)
	return t, err
}

// question is the text a user line asks, if it starts a page: typed text
// (or a slash command), not tool results, meta lines or summaries.
func question(l tline) (string, bool) {
	if l.Type != "user" || l.IsSidechain || l.IsMeta || l.IsCompactSummary {
		return "", false
	}
	var m tmessage
	if json.Unmarshal(l.Message, &m) != nil {
		return "", false
	}
	var s string
	if json.Unmarshal(m.Content, &s) == nil {
		s = strings.TrimSpace(s)
		if s == "" || strings.HasPrefix(s, "<local-command-") || strings.HasPrefix(s, "[Request interrupted") {
			return "", false
		}
		return s, true
	}
	var parts []string
	for _, b := range blocks(m.Content) {
		switch b.Type {
		case "tool_result":
			return "", false
		case "text":
			if t := strings.TrimSpace(b.Text); t != "" && !strings.HasPrefix(t, "[Request interrupted") {
				parts = append(parts, t)
			}
		}
	}
	if len(parts) == 0 {
		return "", false
	}
	return strings.Join(parts, "\n\n"), true
}

var (
	commandNameRE = regexp.MustCompile(`(?s)<command-name>(.*?)</command-name>`)
	commandArgsRE = regexp.MustCompile(`(?s)<command-args>(.*?)</command-args>`)
	commandOutRE  = regexp.MustCompile(`(?s)^<local-command-stdout>(.*)</local-command-stdout>$`)
)

func isCommand(q string) bool { return strings.HasPrefix(q, "<command-name>") }

// isNotification: the CLI's note to itself that a background task finished.
func isNotification(q string) bool { return strings.HasPrefix(q, "<task-notification") }

// commandLine is a slash command as typed: "/compact", "/model sonnet".
func commandLine(q string) string {
	name := ""
	if m := commandNameRE.FindStringSubmatch(q); m != nil {
		name = strings.TrimSpace(m[1])
	}
	if m := commandArgsRE.FindStringSubmatch(q); m != nil && strings.TrimSpace(m[1]) != "" {
		name += " " + strings.TrimSpace(m[1])
	}
	if name == "" {
		return q
	}
	return name
}

func commandOutput(s string) (string, bool) {
	m := commandOutRE.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return "", false
	}
	return strings.TrimSpace(m[1]), true
}

func oneLine(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

func stamp(s string) int64 {
	if s == "" {
		return 0
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return 0
	}
	return t.UnixMilli()
}

// eachLine calls fn for every line that parses; others are skipped.
func eachLine(r io.Reader, fn func(tline)) error {
	sc := bufio.NewScanner(io.LimitReader(r, maxTranscript))
	sc.Buffer(make([]byte, 1<<20), 32<<20)
	for sc.Scan() {
		var l tline
		if json.Unmarshal(sc.Bytes(), &l) == nil {
			fn(l)
		}
	}
	return sc.Err()
}

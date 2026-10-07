package session

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"uncli/internal/core"
	"uncli/internal/store"
)

// Judging: in Ask mode, with the UnknownModel setting, a command UNCLI
// doesn't recognise is judged by the quick-task model before it is
// answered. The request waits without a card meanwhile (the CLI waits
// too), the verdict is stored so each command is judged once, and every
// session then looks again at what it has waiting. If the model can't
// judge it, the user is asked, as with UnknownAsk.

// JudgeFunc asks the decision model about one command part or tool.
type JudgeFunc func(ctx context.Context, q JudgeQuery) (store.Judgement, error)

// judgeTimeout bounds one judgement; past it the user is asked instead.
const judgeTimeout = 90 * time.Second

type judging struct {
	mu       sync.Mutex
	inFlight map[string]bool
	failed   map[string]bool // in memory: after a restart it is tried again
}

func (j *judging) isFailed(key string) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.failed[key]
}

// unknownSetting is what Ask mode does with commands it doesn't recognise.
func (m *Manager) unknownSetting() string {
	if m.d.Unknown != nil {
		if v := m.d.Unknown(); ValidUnknown(v) {
			return v
		}
	}
	if m.d.Judge == nil {
		return UnknownAsk
	}
	return UnknownModel
}

// startJudging asks the model about each query not already being judged,
// in the background, and looks again at every session's waiting requests
// as each answer comes in.
func (m *Manager) startJudging(queries []JudgeQuery) {
	for _, q := range queries {
		m.judge.mu.Lock()
		busy := m.judge.inFlight[q.Key]
		if !busy {
			m.judge.inFlight[q.Key] = true
		}
		m.judge.mu.Unlock()
		if busy {
			continue
		}
		go func(q JudgeQuery) {
			m.judgeNow(q)
			m.rejudgeAll()
		}(q)
	}
}

// judgeNow asks the model about one query and records the answer (or
// that it couldn't).
func (m *Manager) judgeNow(q JudgeQuery) {
	ok := false
	if m.d.Judge != nil {
		ctx, cancel := context.WithTimeout(context.Background(), judgeTimeout)
		j, err := m.d.Judge(ctx, q)
		cancel()
		if err == nil && (j.Level == RiskLooks || j.Level == RiskRoutine || j.Level == RiskRisky) {
			ok = m.d.Store.SetJudgement(q.Key, j) == nil
		}
	}
	m.judge.mu.Lock()
	delete(m.judge.inFlight, q.Key)
	if !ok {
		m.judge.failed[q.Key] = true
	}
	m.judge.mu.Unlock()
}

// judged is an earlier judgement, if the current decision model gave it.
// Judgements from before decision models were the quick-task model's.
func (m *Manager) judged(key string) (store.Judgement, bool) {
	j, ok := m.d.Store.Judgement(key)
	if !ok || m.d.DeciderKey == nil {
		return j, ok
	}
	want := m.d.DeciderKey()
	switch {
	case want == "" || j.Decider == want:
		return j, true
	case j.Decider == "":
		return j, strings.HasPrefix(want, "quick-task/")
	}
	return store.Judgement{}, false
}

// waitingForUserLocked reports whether a card is waiting for the user
// (not just for the model). Caller holds s.mu.
func (s *Session) waitingForUserLocked() bool {
	for _, a := range s.pending {
		if !a.Judging {
			return true
		}
	}
	return false
}

// JudgeToolsFunc asks the decision model about several tools of one MCP
// server at once, returning a judgement for each tool it could judge.
type JudgeToolsFunc func(ctx context.Context, q ToolsQuery) (map[string]store.Judgement, error)

// ToolsQuery is a server's tools that gave no hints, to judge together.
type ToolsQuery struct {
	Server  string   `json:"server"`
	Version string   `json:"version,omitempty"`
	Tools   []string `json:"tools"` // the CLI's names (mcp__server__tool)
	Workdir string   `json:"workdir"`
}

// prejudge judges each MCP server's tools that gave no hints as soon as
// the CLI reports them, one decision per server, rather than one per tool
// as each is first used. Tools already judged (for this server version),
// or being judged, are left out. If a batch fails, each tool is still
// judged on its own when it is used.
func (m *Manager) prejudge(hints map[string]core.ToolHint, workdir string) {
	if m.d.JudgeTools == nil || m.unknownSetting() != UnknownModel {
		return
	}
	byServer := map[string]*ToolsQuery{}
	for tool, h := range hints {
		if h.ReadOnly || h.Destructive || h.OpenWorld || h.Server == "" {
			continue // the server said what it does
		}
		key := mcpJudgeKey(tool, h)
		if _, ok := m.judged(key); ok {
			continue
		}
		m.judge.mu.Lock()
		skip := m.judge.inFlight[key] || m.judge.failed[key]
		if !skip {
			m.judge.inFlight[key] = true
		}
		m.judge.mu.Unlock()
		if skip {
			continue
		}
		q := byServer[h.Server]
		if q == nil {
			q = &ToolsQuery{Server: h.Server, Version: h.ServerVersion, Workdir: workdir}
			byServer[h.Server] = q
		}
		q.Tools = append(q.Tools, tool)
	}
	for _, q := range byServer {
		sort.Strings(q.Tools)
		go m.judgeServer(*q)
	}
}

func (m *Manager) judgeServer(q ToolsQuery) {
	ctx, cancel := context.WithTimeout(context.Background(), judgeTimeout)
	got, err := m.d.JudgeTools(ctx, q)
	cancel()
	for _, tool := range q.Tools {
		key := mcpJudgeKey(tool, core.ToolHint{Server: q.Server, ServerVersion: q.Version})
		if j, ok := got[tool]; err == nil && ok && (j.Level == RiskLooks || j.Level == RiskRoutine || j.Level == RiskRisky) {
			_ = m.d.Store.SetJudgement(key, j)
		}
		m.judge.mu.Lock()
		delete(m.judge.inFlight, key)
		m.judge.mu.Unlock()
	}
	m.rejudgeAll()
}

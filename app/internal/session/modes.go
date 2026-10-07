package session

import (
	"encoding/json"
	"strings"

	"uncli/internal/core"
	"uncli/internal/store"
)

// When to prompt: the session's level decides UNCLI's answer to each tool
// use the CLI asks about, live, from whether it is safe (risk.go):
//
//	always  Always prompt me: everything but reading prompts
//	unsafe  Prompt when unsafe (the default): reading and safe work run;
//	        unsafe work prompts; what UNCLI can't place follows the user's
//	        setting (UnknownModel, UnknownInside, UnknownAsk)
//	never   Run without prompting: everything runs (never the CLI's bypass
//	        mode: the CLI still asks UNCLI, and blocked entries still block)
//
// Unattended (approvals.go) declines whatever would prompt.
//
// The user's safe list (class.go) corrects UNCLI: an entry marks a command
// class or tool safe, unsafe or blocked, in this project or all of them.
// It beats the session type's list, the built-in tables and the model, but
// not what makes one use unsafe (context risk). Questions to the user
// (AskUserQuestion, ExitPlanMode) always go to them.
const (
	ModeAlways = "always"
	ModeUnsafe = "unsafe"
	ModeNever  = "never"
)

// What "Prompt when unsafe" does with a command it can't place.
const (
	UnknownModel  = "model"  // the quick-task model judges it, once per class
	UnknownInside = "inside" // it runs, unless it works outside the folder
	UnknownAsk    = "ask"    // the user is prompted
)

// ValidUnknown reports whether v names a setting for unrecognised commands.
func ValidUnknown(v string) bool { return v == UnknownModel || v == UnknownInside || v == UnknownAsk }

// ValidMode reports whether m names a prompt level.
func ValidMode(m string) bool { return m == ModeAlways || m == ModeUnsafe || m == ModeNever }

// modeOf reads a stored level; empty is Prompt when unsafe.
func modeOf(m string) string {
	if ValidMode(m) {
		return m
	}
	return ModeUnsafe
}

const deniedByUser = "Blocked: the user has blocked this command in UNCLI. Don't retry it or look for another way to do it; say what you wanted to do instead."

// policy is everything a decision depends on.
type policy struct {
	mode    string
	unknown string // UnknownModel, UnknownInside or UnknownAsk
	safe    []store.SafeEntry
	profile []allowEntry // the session type's list
	workdir string
	cwd     string   // the folder the shell is in, if Claude has moved it (cd) within workdir
	state   []string // folders the CLI keeps its own files in (core.StatePather)
	hints   map[string]core.ToolHint
	judged  func(key string) (store.Judgement, bool) // earlier model judgements
	failed  func(key string) bool                    // the model couldn't judge it
}

func (p policy) place() place { return place{workdir: p.workdir, state: p.state} }

func (p policy) cwdOrWorkdir() string {
	if p.cwd != "" {
		return p.cwd
	}
	return p.workdir
}

// actionJudge: wait for the quick-task model to judge something, then decide.
const actionJudge = "judge"

// Actions of a verdict.
const (
	actionRun    = "allow"
	actionPrompt = "ask"
	actionBlock  = "deny"
)

// verdict is UNCLI's answer to one tool use.
type verdict struct {
	action string // actionRun, actionPrompt, actionBlock or actionJudge
	by     string // why it ran, for the trace: looks, safe, listed, builtin, inside, never or mixed
	reason string // why it was blocked, for the model
	why    []store.TraceReason
	class  Class
	judge  []JudgeQuery // what the model must judge first (actionJudge)
}

// JudgeQuery is something for the quick-task model to judge: one part of a
// command line, or a tool UNCLI knows nothing about.
type JudgeQuery struct {
	Key         string          `json:"key"` // cache key
	Tool        string          `json:"tool"`
	Part        string          `json:"part,omitempty"`    // the part of the command line to judge
	Command     string          `json:"command,omitempty"` // the whole command line, for context
	Code        string          `json:"code,omitempty"`    // code fed to the part (a heredoc)
	Dialect     string          `json:"dialect,omitempty"` // bash or powershell, for a command
	Input       json.RawMessage `json:"input,omitempty"`   // a tool's input
	Description string          `json:"description,omitempty"`
	Workdir     string          `json:"workdir"`
}

// keyFor is the cache key for judging one check. An MCP tool's judgement
// holds for the server version that reported it, so a server update judges
// its tools again (the CLI gives no tool descriptions or schemas to
// compare; BRUV card "Judge MCP tools per server").
func (p policy) keyFor(a core.ToolAction, text string, class *store.SafeClass) string {
	if a.Kind == core.ActMCP {
		return mcpJudgeKey(a.Tool, p.hints[a.Tool])
	}
	return judgeKey(a, text, class)
}

func mcpJudgeKey(tool string, h core.ToolHint) string {
	return "mcp:" + tool + "@" + h.Server + "/" + h.ServerVersion
}

// judgeKey is the cache key for a judgement: the class when one can be
// learned (so every run of it is judged once), else the part as written.
func judgeKey(a core.ToolAction, text string, class *store.SafeClass) string {
	switch {
	case a.Kind != core.ActShell:
		return "tool:" + a.Tool
	case class != nil:
		return "class:" + class.Words + "|" + class.Flags
	}
	return "cmd:" + strings.Join(strings.Fields(text), " ")
}

// check is one thing to decide: the tool, or one working part of a command.
type check struct {
	text    string   // the part as written; empty for a whole tool use
	words   []string // the part's words; nil for a tool or an unreadable command
	risk    Risk     // what it could do, all things considered
	context bool     // risk comes from where it works (can't be learned or overridden)
	class   store.SafeClass
	fixed   string // why its class can't be learned
	code    string // code fed to it (a heredoc), for the model to read
}

// judge answers a tool use. Each check gets a reason of its own; the answer
// is the strictest: blocked, then waiting for the model, then prompting;
// otherwise it runs.
func (p policy) judge(a core.ToolAction) verdict {
	cls := classify(a, p.hints)
	var checks []check
	cmd := ""
	if a.Kind == core.ActShell {
		cmd = strings.TrimSpace(a.Command)
		parts, ok := workingParts(cmd, a.Dialect, p.cwdOrWorkdir())
		if !ok { // can't be read at all: judged whole
			checks = []check{{text: cmd, risk: unknown, fixed: FixedComplex}}
		}
		for _, pt := range parts {
			prog, ctx := partRisk(pt, p.place())
			c := check{text: strings.TrimSpace(pt.text), words: pt.words, risk: prog, code: pt.body}
			if ctx.Level == RiskRisky {
				c.risk, c.context = ctx, true
			}
			c.class, c.fixed = classOf(pt)
			if c.context && c.fixed == "" {
				c.fixed = FixedContext
			}
			checks = append(checks, c)
		}
	} else {
		r := toolRisk(a, cls, p.place())
		c := check{risk: r, class: store.SafeClass{Kind: store.KindTool, Words: a.Tool}}
		if fileAction(a) && (r.Why == WhyOutside || r.Why == WhySecrets) {
			c.context, c.fixed = true, FixedContext
		}
		checks = []check{c}
	}
	var why []store.TraceReason
	var queries []JudgeQuery
	for _, c := range checks {
		r := p.reason(a, c)
		if r.By == "judging" {
			q := JudgeQuery{Key: p.keyFor(a, c.text+c.code, r.Class), Tool: a.Tool, Part: c.text, Command: cmd, Code: c.code, Workdir: p.workdir}
			if a.Kind == core.ActShell {
				q.Dialect = a.Dialect
			} else {
				q.Input = a.Input
			}
			queries = append(queries, q)
		}
		why = append(why, r)
	}
	v := combine(why, cls)
	v.judge = queries
	return v
}

// reason decides one check.
func (p policy) reason(a core.ToolAction, c check) store.TraceReason {
	mode := modeOf(p.mode)
	at := store.TraceReason{Part: c.text, Fixed: c.fixed}
	if c.fixed == "" {
		cl := c.class
		at.Class = &cl
	}
	by := func(b string) store.TraceReason { at.By = b; return at }

	// What the user's safe list says, and what that makes it.
	risk := c.risk
	source := ""
	e, listed := store.SafeEntry{}, false
	if c.fixed != FixedComplex {
		e, listed = matchEntry(p.safe, a, c.class, c.words)
	}
	switch {
	case listed && e.Verdict == store.Blocked:
		at.Entry = &e
		return by("blocked")
	case asksUser(a):
		return by("user")
	case listed && !c.context && c.fixed != FixedInline:
		at.Entry = &e
		source = "listed"
		if e.Verdict == store.Safe {
			if risk.Level != RiskLooks {
				risk = routine
			}
		} else {
			risk = risky("")
		}
	case !c.context && c.fixed == "" && risk.Level != RiskLooks:
		if entry, ok := allowEntryFor(p.profile, a, c.words, p.workdir); ok {
			at.Allow, source, risk = entry, "builtin", routine
		}
	}

	switch {
	case mode == ModeNever:
		return by("never")
	case risk.Level == RiskLooks:
		return by("looks")
	case mode == ModeAlways:
		return by("always")
	case risk.Level == RiskRoutine:
		if source == "" {
			source = "safe"
		}
		return by(source)
	case risk.Level == RiskRisky:
		at.Risk = risk.Why
		return by("unsafe")
	}

	// Not recognised: the user's setting decides.
	switch p.unknown {
	case UnknownInside:
		return by("inside")
	case UnknownModel:
		key := p.keyFor(a, c.text+c.code, at.Class)
		if p.judged != nil {
			if j, ok := p.judged(key); ok {
				at.Judged, at.Note = j.Model, j.Note
				switch j.Level {
				case RiskLooks:
					return by("looks")
				case RiskRoutine:
					return by("safe")
				}
				at.Risk = j.Risk
				return by("unsafe")
			}
		}
		if p.failed == nil || !p.failed(key) {
			return by("judging")
		}
	}
	return by("unknown")
}

// combine turns the reasons for each check into one answer.
func combine(why []store.TraceReason, cls Class) verdict {
	v := verdict{action: actionPrompt, why: why, class: cls}
	has := map[string]bool{}
	for _, r := range why {
		has[r.By] = true
	}
	switch {
	case has["blocked"]:
		v.action, v.reason = actionBlock, deniedByUser
	case has["judging"]:
		v.action = actionJudge
	case has["user"], has["always"], has["unsafe"], has["unknown"]:
	default:
		v.action = actionRun
		for _, r := range why {
			switch {
			case v.by == "":
				v.by = r.By
			case v.by != r.By:
				v.by = "mixed"
			}
		}
	}
	return v
}

// learnable are the classes "This is safe" would remember for a waiting
// request: one for each part that prompts. ok is false when a part that
// prompts can't be learned (or is a question to the user).
func learnable(why []store.TraceReason) (classes []store.SafeClass, ok bool) {
	for _, r := range why {
		switch r.By {
		case "unsafe", "unknown", "always":
			if r.Class == nil {
				return nil, false
			}
			classes = append(classes, *r.Class)
		case "user":
			return nil, false
		}
	}
	return classes, len(classes) > 0
}

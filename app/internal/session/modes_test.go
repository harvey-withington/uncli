package session

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"uncli/internal/core"
	"uncli/internal/store"
)

// The MCP server of the mcp-status-tool-hints fixture: annotated and
// unannotated tools, as the CLI reports them.
var notesHints = map[string]core.ToolHint{
	"mcp__notes__read_note":   {ReadOnly: true},
	"mcp__notes__delete_note": {Destructive: true},
	"mcp__notes__lookup_web":  {ReadOnly: true, OpenWorld: true},
	"mcp__notes__touch_note":  {},
}

func cmd(c string) json.RawMessage { return bash(c) }

func TestClassify(t *testing.T) {
	cases := []struct {
		tool  string
		input json.RawMessage
		want  Class
	}{
		{"Read", nil, Class{Source: SourceBuiltIn}},
		{"WebFetch", nil, Class{OpenWorld: true, Source: SourceBuiltIn}},
		{"Edit", nil, Class{Write: true, Source: SourceBuiltIn}},
		{"mcp__notes__read_note", nil, Class{Source: SourceServer}},
		{"mcp__notes__delete_note", nil, Class{Write: true, Destructive: true, Source: SourceServer}},
		{"mcp__notes__touch_note", nil, Class{Write: true, Source: SourceServer}},
		{"mcp__other__thing", nil, Class{Write: true, Destructive: true, OpenWorld: true, Source: SourceUnknown}},
		{"Bash", cmd("git status && ls | grep go"), Class{Source: SourceBuiltIn}},
		{"Bash", cmd("rm -rf build"), Class{Write: true, Destructive: true, Source: SourceBuiltIn}},
		{"Bash", cmd("git push origin main"), Class{Write: true, OpenWorld: true, Source: SourceBuiltIn}},
	}
	for _, c := range cases {
		if got := classify(action(c.tool, c.input), notesHints); got != c.want {
			t.Errorf("%s %s = %+v, want %+v", c.tool, c.input, got, c.want)
		}
	}
}

func TestAllowlist(t *testing.T) {
	dir := t.TempDir()
	list := parseAllowlist([]string{"Write(./artifacts/**)", "Bash(git status:*)", "Bash(npm test)", "Read", "WebFetch(domain:example.com)", "nonsense("}, dir)
	file := func(p string) json.RawMessage {
		b, _ := json.Marshal(map[string]string{"file_path": p})
		return b
	}
	cases := []struct {
		tool  string
		words []string
		input json.RawMessage
		want  bool
	}{
		{"Write", nil, file(filepath.Join(dir, "artifacts", "a", "page.html")), true},
		{"Write", nil, file("artifacts/page.html"), true}, // relative to the session folder
		{"Write", nil, file(filepath.Join(dir, "page.html")), false},
		{"Write", nil, file(filepath.Join(dir, "..", "artifacts", "x")), false},
		{"Edit", nil, file(filepath.Join(dir, "artifacts", "x")), false},
		{"Read", nil, file("/anywhere"), true},
		{"PowerShell", []string{"git", "status", "-s"}, nil, true}, // a Bash entry covers both shells
		{"Bash", []string{"git", "stash"}, nil, false},
		{"Bash", []string{"npm", "test"}, nil, true},
		{"Bash", []string{"npm", "test", "--", "-u"}, nil, false}, // exact
		{"WebFetch", nil, nil, false},
	}
	for _, c := range cases {
		if got := allowedBy(list, action(c.tool, c.input), c.words, dir); got != c.want {
			t.Errorf("%s %v %s = %v, want %v", c.tool, c.words, c.input, got, c.want)
		}
	}
	if g := globRegexp("C:/a/**/*.go"); !g.MatchString("C:/a/b/c/x.go") || !g.MatchString("C:/a/x.go") || g.MatchString("C:/a/b/x.goo") {
		t.Errorf("glob %s", g)
	}
}

// The three levels, against reading, safe work, unsafe work and what
// UNCLI can't place; then the safe list and the session type's list.
func TestJudgeLevels(t *testing.T) {
	type want struct{ action, by string }
	cases := []struct {
		name, tool, cmd string
		levels          [3]want // always, unsafe, never
	}{
		{"reading", "Bash", "git status --short; Get-Content a.txt | Select-Object -First 3",
			[3]want{{actionRun, "looks"}, {actionRun, "looks"}, {actionRun, "never"}}},
		{"read tool", "Read", "",
			[3]want{{actionRun, "looks"}, {actionRun, "looks"}, {actionRun, "never"}}},
		{"tests", "PowerShell", `npm --prefix "packages\core" test; npm --prefix "packages\core" run lint`,
			[3]want{{actionPrompt, ""}, {actionRun, "safe"}, {actionRun, "never"}}},
		{"local git", "Bash", "git add -A && git commit -m wip",
			[3]want{{actionPrompt, ""}, {actionRun, "safe"}, {actionRun, "never"}}},
		{"edit", "Edit", "",
			[3]want{{actionPrompt, ""}, {actionRun, "safe"}, {actionRun, "never"}}},
		{"push", "Bash", "git push",
			[3]want{{actionPrompt, ""}, {actionPrompt, ""}, {actionRun, "never"}}},
		{"recursive delete", "PowerShell", "Remove-Item -Recurse src",
			[3]want{{actionPrompt, ""}, {actionPrompt, ""}, {actionRun, "never"}}},
		{"unannotated MCP tool", "mcp__notes__touch_note", "",
			[3]want{{actionPrompt, ""}, {actionPrompt, ""}, {actionRun, "never"}}},
		{"annotated read", "mcp__notes__read_note", "",
			[3]want{{actionRun, "looks"}, {actionRun, "looks"}, {actionRun, "never"}}},
		{"not recognised", "Bash", "frobnicate --all",
			[3]want{{actionPrompt, ""}, {actionPrompt, ""}, {actionRun, "never"}}},
		// A called variable runs whatever it holds: never just a value.
		{"called variable", "PowerShell", `$p = Get-Content prog.txt; & $p`,
			[3]want{{actionPrompt, ""}, {actionPrompt, ""}, {actionRun, "never"}}},
		// sed -i edits: safe work in the folder, never reading.
		{"sed in place", "Bash", `sed -i 's/a/b/' src/a.go`,
			[3]want{{actionPrompt, ""}, {actionRun, "safe"}, {actionRun, "never"}}},
		{"sed in place outside", "Bash", `sed -i 's/a/b/' /etc/hosts`,
			[3]want{{actionPrompt, ""}, {actionPrompt, ""}, {actionRun, "never"}}},
		{"question", "AskUserQuestion", "",
			[3]want{{actionPrompt, ""}, {actionPrompt, ""}, {actionPrompt, ""}}},
	}
	for _, c := range cases {
		for i, mode := range []string{ModeAlways, ModeUnsafe, ModeNever} {
			p := policy{mode: mode, unknown: UnknownAsk, workdir: "/repo", hints: notesHints}
			var in json.RawMessage
			if c.cmd != "" {
				in = cmd(c.cmd)
			} else if c.tool == "Edit" || c.tool == "Read" {
				in = json.RawMessage(`{"file_path":"src/a.go"}`)
			}
			v := p.judgeTool(c.tool, in)
			if w := c.levels[i]; v.action != w.action || v.by != w.by {
				t.Errorf("%s, %s: %s/%q, want %s/%q (%+v)", c.name, mode, v.action, v.by, w.action, w.by, v.why)
			}
		}
	}
	// Questions to the user and empty levels.
	if modeOf("") != ModeUnsafe || modeOf("readonly") != ModeUnsafe {
		t.Error("an unknown level is Prompt when unsafe")
	}
}

func TestJudgeSafeList(t *testing.T) {
	entry := func(words, flags, verdict, folder string) store.SafeEntry {
		return store.SafeEntry{Kind: store.KindCommand, Words: words, Flags: flags, Verdict: verdict, Folder: folder}
	}
	p := policy{mode: ModeUnsafe, unknown: UnknownAsk, workdir: "/repo", hints: notesHints, safe: []store.SafeEntry{
		entry("git push", "", store.Safe, "/repo"),
		entry("npm run deploy", "", store.Blocked, ""),
		entry("npm run test", "", store.Unsafe, ""),
		entry("frobnicate", "", store.Safe, ""),
		entry("git:local", "", store.Unsafe, ""),
		{Kind: store.KindTool, Words: "mcp__notes__touch_note", Verdict: store.Safe},
		entry("cp", "", store.Safe, ""),
	}}
	cases := []struct {
		tool, cmd, action, by string
	}{
		{"Bash", "git push origin main", actionRun, "listed"},
		{"Bash", "git push --force", actionPrompt, ""}, // risk flags make another class
		{"Bash", "npm run deploy -- --prod", actionBlock, ""},
		{"Bash", "npm run test -- -u src/a.test.ts", actionPrompt, ""}, // the user says unsafe
		{"Bash", "frobnicate --all", actionRun, "listed"},
		{"Bash", "git switch main", actionPrompt, ""}, // a git class entry
		{"mcp__notes__touch_note", "", actionRun, "listed"},
		{"Bash", "cp a.txt ../elsewhere/", actionPrompt, ""}, // outside the folder: the list can't override that
	}
	for _, c := range cases {
		var in json.RawMessage
		if c.cmd != "" {
			in = cmd(c.cmd)
		}
		v := p.judgeTool(c.tool, in)
		if v.action != c.action || v.by != c.by {
			t.Errorf("%s: %s/%q, want %s/%q (%+v)", c.cmd, v.action, v.by, c.action, c.by, v.why)
		}
	}
	// Blocked holds under Run without prompting too; Safe doesn't skip
	// Always prompt me.
	p.mode = ModeNever
	if v := p.judgeTool("Bash", cmd("npm run deploy")); v.action != actionBlock {
		t.Errorf("never + blocked = %s", v.action)
	}
	p.mode = ModeAlways
	if v := p.judgeTool("Bash", cmd("frobnicate")); v.action != actionPrompt {
		t.Errorf("always + safe entry = %s", v.action)
	}
	// This project's entry wins a tie with all projects'.
	p.mode = ModeUnsafe
	p.safe = []store.SafeEntry{entry("make", "", store.Unsafe, ""), entry("make", "", store.Safe, "/repo")}
	if v := p.judgeTool("Bash", cmd("make docs")); v.action != actionRun {
		t.Errorf("project beats all projects: %s", v.action)
	}
}

// Each part says why, and what "This is safe" would remember.
func TestJudgeWhy(t *testing.T) {
	p := policy{mode: ModeUnsafe, unknown: UnknownAsk, workdir: "/repo",
		profile: parseAllowlist([]string{"Bash(npm test:*)"}, "/repo")}
	v := p.judgeTool("PowerShell", cmd("git status --short; npm test; git push --force; node -e \"x()\""))
	got := []string{}
	for _, r := range v.why {
		c := ""
		if r.Class != nil {
			c = r.Class.Words + "|" + r.Class.Flags
		}
		got = append(got, r.By+":"+c+":"+r.Fixed)
	}
	want := []string{"looks:git status|:", "builtin:npm test|:", "unsafe:git push|--force:", "unknown::inline"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("why = %v", got)
	}
	if _, ok := learnable(v.why); ok {
		t.Error("inline code can't be learned, so neither can the whole command")
	}
	v = p.judgeTool("Bash", cmd("git push; npm run lint; npm run deploy")) // a deploy script publishes; lint is safe
	classes, ok := learnable(v.why)
	if !ok || len(classes) != 2 || classes[1].Words != "npm run deploy" {
		t.Errorf("learn = %+v %v", classes, ok)
	}
}

// What can't be placed follows the user's setting.
func TestJudgeUnknown(t *testing.T) {
	p := policy{mode: ModeUnsafe, workdir: "/repo"}
	run := func(c string) verdict { return p.judgeTool("PowerShell", cmd(c)) }
	p.unknown = UnknownAsk
	if v := run("frobnicate --all"); v.action != actionPrompt || v.why[0].By != "unknown" {
		t.Errorf("ask: %s %+v", v.action, v.why)
	}
	p.unknown = UnknownInside
	if v := run("frobnicate --all"); v.action != actionRun || v.by != "inside" {
		t.Errorf("inside: %s %+v", v.action, v.why)
	}
	if v := run("frobnicate C:/Windows/System32"); v.action != actionPrompt || v.why[0].Risk != WhyOutside {
		t.Errorf("inside, but names a path outside: %s %+v", v.action, v.why)
	}
	p.unknown = UnknownModel
	v := run("git status; frobnicate sync --all")
	if v.action != actionJudge || len(v.judge) != 1 || v.judge[0].Key != "class:frobnicate sync|" {
		t.Fatalf("model: %s %+v", v.action, v.judge)
	}
	judged := map[string]store.Judgement{"class:frobnicate sync|": {Level: RiskRoutine, Note: "Syncs the docs.", Model: "haiku"}}
	p.judged = func(k string) (store.Judgement, bool) { j, ok := judged[k]; return j, ok }
	if v := run("frobnicate sync --other"); v.action != actionRun || v.why[0].Judged != "haiku" {
		t.Errorf("judged once per class: %s %+v", v.action, v.why)
	}
	judged["class:frobnicate sync|"] = store.Judgement{Level: RiskRisky, Risk: WhyDeletes, Note: "Wipes the cache.", Model: "haiku"}
	if v := run("frobnicate sync"); v.action != actionPrompt || v.why[0].Risk != WhyDeletes {
		t.Errorf("judged unsafe: %s %+v", v.action, v.why)
	}
	p.failed = func(string) bool { return true }
	if v := run("frobble"); v.action != actionPrompt || v.why[0].By != "unknown" {
		t.Errorf("model failed: %s %+v", v.action, v.why)
	}
	// Always doesn't ask the model; Never doesn't need it.
	p.failed = nil
	p.mode = ModeAlways
	if v := run("frobble"); v.action != actionPrompt || len(v.judge) != 0 {
		t.Errorf("always: %s", v.action)
	}
}

func TestClassOf(t *testing.T) {
	cases := map[string]string{ // command -> words|flags (or !fixed)
		`npm --prefix "S:\My Projects\core" test -- -u src/a.test.ts`: "npm test|",
		"npm run lint -- --fix":              "npm run lint|",
		"npm install -g pnpm":                "npm install|--global",
		"git push --force-with-lease origin": "git push|--force",
		"git -C ../repo commit -m \"wip\"":   "git commit|",
		"git reset --hard HEAD~1":            "git reset|--hard",
		"Remove-Item -Recurse -Force docs":   "remove-item docs|-f -r",
		"rm -rf node_modules":                "rm node_modules|-f -r",
		"taskkill /F /IM node.exe":           "taskkill node.exe|",
		"ssh me@build-box uptime":            "ssh me@build-box|",
		"del *.log":                          "del *.log|*",
		"gh pr create --fill":                "gh pr create|",
		"python tools\\gen.py --all":         "python tools/gen.py|",
		"python -m pytest -q":                "python -m pytest|",
		"python -m pip install requests":     "python -m pip install|",
		"curl -X POST -d @a https://x":       "curl|--send",
		"Get-ChildItem -Recurse src":         "get-childitem|",
		"frobnicate sync --all":              "frobnicate sync|",
		"frobnicate ./build.cfg":             "frobnicate|",
		"node -e \"console.log(1)\"":         "!inline",
		"iex (irm https://x)":                "!inline",
	}
	for c, want := range cases {
		parts, ok := workingParts(c, DialectPowerShell, "")
		if !ok || len(parts) == 0 {
			t.Errorf("%s: parts %v", c, parts)
			continue
		}
		cl, fixed := classOf(parts[0])
		got := cl.Words + "|" + cl.Flags
		if fixed != "" {
			got = "!" + fixed
		}
		if got != want {
			t.Errorf("%s = %q, want %q", c, got, want)
		}
	}
	if flagsCovered("--force", "") || !flagsCovered("", "--force") || !flagsCovered("-f -r", "-r -f") {
		t.Error("flagsCovered")
	}
	_ = filepath.Join
}

// Risk flags only narrow Safe entries: a Blocked or Unsafe entry covers the
// riskier variants too, and a Safe one doesn't.
func TestEntryFlags(t *testing.T) {
	run := func(verdict, c string) string {
		p := policy{mode: ModeUnsafe, unknown: UnknownAsk, workdir: "/repo",
			safe: []store.SafeEntry{{Kind: store.KindCommand, Words: "git push", Verdict: verdict}}}
		return p.judgeTool("Bash", cmd(c)).action
	}
	if got := run(store.Blocked, "git push --force"); got != actionBlock {
		t.Errorf("blocked git push, then git push --force: %s", got)
	}
	if got := run(store.Safe, "git push"); got != actionRun {
		t.Errorf("safe git push: %s", got)
	}
	if got := run(store.Safe, "git push --force"); got != actionPrompt {
		t.Errorf("safe git push, then git push --force: %s", got)
	}
}

package antigravity

import (
	"encoding/json"
	"runtime"
	"strings"

	"uncli/internal/core"
)

// What the Antigravity CLI's tools do, as provider-neutral actions
// (core.ToolAction). Argument names are those seen in the recorded hook
// calls (testdata/streams/antigravity/<version>/*.hook.jsonl); tools not yet
// seen are read defensively and fall back to "other", which prompts.

var toolKinds = map[string]string{
	"run_command": core.ActShell,
	"view_file":   core.ActRead, "command_status": core.ActRead, "read_resource": core.ActRead,
	"list_dir": core.ActSearch, "grep_search": core.ActSearch, "find_by_name": core.ActSearch, "list_resources": core.ActSearch,
	"write_to_file":        core.ActWrite,
	"replace_file_content": core.ActEdit, "multi_replace_file_content": core.ActEdit, "sed_file": core.ActEdit, "notebook_edit": core.ActEdit,
	"read_url_content": core.ActWeb, "search_web": core.ActWeb,
	"call_mcp_tool": core.ActMCP,
	"ask_question":  core.ActQuestion, "ask_permission": core.ActQuestion, "ask_custom_permission": core.ActQuestion,
	"invoke_subagent": core.ActAgent, "define_subagent": core.ActAgent, "manage_subagents": core.ActAgent, "browser_subagent": core.ActAgent,
	"send_message": core.ActInternal, "wait": core.ActInternal, "wait_5_seconds": core.ActInternal, "finish": core.ActInternal,
	"manage_task": core.ActInternal, "manage_inbox": core.ActInternal, "list_permissions": core.ActInternal,
}

// shellDialect is the shell run_command uses: PowerShell on Windows
// (recorded), a POSIX shell elsewhere.
func shellDialect() string {
	if runtime.GOOS == "windows" {
		return "powershell"
	}
	return "bash"
}

// ActionOf says what an Antigravity tool call does.
func ActionOf(tool string, args json.RawMessage) core.ToolAction {
	a := core.ToolAction{Kind: toolKinds[tool], Tool: tool, Input: args}
	if a.Kind == "" {
		a.Kind = core.ActOther
	}
	var in map[string]any
	_ = json.Unmarshal(args, &in)
	str := func(keys ...string) string {
		for _, k := range keys {
			if s, ok := in[k].(string); ok && s != "" {
				return s
			}
		}
		return ""
	}
	switch a.Kind {
	case core.ActShell:
		a.Command = str("CommandLine")
		a.Dialect = shellDialect()
	case core.ActRead, core.ActWrite, core.ActEdit, core.ActSearch:
		a.Path = str("AbsolutePath", "TargetFile", "File", "FilePath", "Path", "DirectoryPath", "SearchPath", "SearchDirectory")
	case core.ActMCP:
		// The tool's identity is its server and name, as the safe list keys it.
		if s, t := str("ServerName", "Server"), str("ToolName", "Tool", "Name"); s != "" && t != "" {
			a.Tool = "mcp__" + s + "__" + t
		}
	}
	return a
}

// summary is the trace strip's one line for a tool call: what the CLI says
// it is doing, or else its main argument.
func summary(tool string, args json.RawMessage) string {
	var in map[string]any
	_ = json.Unmarshal(args, &in)
	for _, k := range []string{"toolAction", "toolSummary", "CommandLine", "AbsolutePath", "TargetFile", "Url", "query"} {
		if s, ok := in[k].(string); ok && strings.TrimSpace(s) != "" {
			return firstLine(s)
		}
	}
	return tool
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i] + " …"
	}
	if r := []rune(s); len(r) > 200 {
		s = string(r[:199]) + "…"
	}
	return s
}

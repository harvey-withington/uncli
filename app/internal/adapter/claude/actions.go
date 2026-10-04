package claude

import (
	"encoding/json"
	"strings"

	"uncli/internal/core"
)

// What Claude Code's tools do, as provider-neutral actions (core.ToolAction).
// Everything UNCLI needs to know about Claude's tool names and inputs to
// decide when to prompt lives here.

var toolKinds = map[string]string{
	"Bash": core.ActShell, "PowerShell": core.ActShell,
	"Read": core.ActRead, "NotebookRead": core.ActRead, "BashOutput": core.ActRead, "ReadMcpResourceTool": core.ActRead,
	"Grep": core.ActSearch, "Glob": core.ActSearch, "LS": core.ActSearch, "ListMcpResourcesTool": core.ActSearch,
	"Write": core.ActWrite,
	"Edit":  core.ActEdit, "MultiEdit": core.ActEdit, "NotebookEdit": core.ActEdit,
	"WebSearch": core.ActWeb, "WebFetch": core.ActWeb,
	"AskUserQuestion": core.ActQuestion, "ExitPlanMode": core.ActPlan,
	"Task": core.ActAgent, "Agent": core.ActAgent, "Workflow": core.ActAgent,
	"TodoWrite": core.ActInternal, "TodoRead": core.ActInternal, "Skill": core.ActInternal, "ToolSearch": core.ActInternal,
	"EnterPlanMode": core.ActInternal, "SubagentHandback": core.ActInternal, "SendMessage": core.ActInternal,
	"ListAgents": core.ActInternal, "TaskCreate": core.ActInternal, "TaskGet": core.ActInternal, "TaskList": core.ActInternal,
	"TaskUpdate": core.ActInternal, "TaskStop": core.ActInternal, "TaskOutput": core.ActInternal, "Monitor": core.ActInternal,
	"ScheduleWakeup": core.ActInternal, "EnterWorktree": core.ActInternal, "ExitWorktree": core.ActInternal,
	"ReportFindings": core.ActInternal, "KillShell": core.ActInternal, "SlashCommand": core.ActInternal,
}

// ActionOf says what a Claude Code tool use does.
func ActionOf(tool string, input json.RawMessage) core.ToolAction {
	a := core.ToolAction{Kind: toolKinds[tool], Tool: tool, Input: input}
	if strings.HasPrefix(tool, "mcp__") {
		a.Kind = core.ActMCP
	}
	if a.Kind == "" {
		a.Kind = core.ActOther
	}
	var in struct {
		Command      string `json:"command"`
		FilePath     string `json:"file_path"`
		NotebookPath string `json:"notebook_path"`
		Path         string `json:"path"`
	}
	_ = json.Unmarshal(input, &in)
	switch a.Kind {
	case core.ActShell:
		a.Command = in.Command
		a.Dialect = "bash"
		if tool == "PowerShell" {
			a.Dialect = "powershell"
		}
	case core.ActRead, core.ActWrite, core.ActEdit, core.ActSearch:
		a.Path = in.FilePath
		if a.Path == "" {
			a.Path = in.NotebookPath
		}
		if a.Path == "" {
			a.Path = in.Path
		}
	}
	return a
}

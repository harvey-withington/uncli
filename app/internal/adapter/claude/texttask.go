package claude

import (
	"encoding/json"
	"errors"
	"strings"

	"uncli/internal/core"
)

// TextTaskCommand runs a single isolated turn: no tools, no user settings,
// MCP servers or skills, nothing saved as a session. The prompt is read
// from stdin.
func (a *Adapter) TextTaskCommand(bin string, t core.TextTask) core.Command {
	args := []string{"-p", "--output-format", "json", "--no-session-persistence",
		"--tools", "", "--strict-mcp-config", "--setting-sources", "", "--disable-slash-commands",
		"--permission-prompts", "none"}
	if t.Model != "" {
		args = append(args, "--model", t.Model)
	}
	if t.System != "" {
		args = append(args, "--system-prompt", t.System)
	}
	if len(t.Schema) > 0 {
		args = append(args, "--json-schema", string(t.Schema))
	}
	env, drop := childEnv(nil)
	return core.Command{Path: bin, Args: args, Env: env, EnvDrop: drop}
}

// ParseTextTask reads the single JSON result "--output-format json" prints.
func (a *Adapter) ParseTextTask(out []byte) (core.TextResult, error) {
	var r struct {
		IsError          bool            `json:"is_error"`
		Result           string          `json:"result"`
		StructuredOutput json.RawMessage `json:"structured_output"`
		TotalCostUSD     float64         `json:"total_cost_usd"`
		Usage            *resultUsage    `json:"usage"`
	}
	if err := json.Unmarshal(out, &r); err != nil {
		return core.TextResult{}, errors.New("the CLI's answer wasn't readable")
	}
	if r.IsError {
		msg := strings.TrimSpace(r.Result)
		if msg == "" {
			msg = "the task failed"
		}
		return core.TextResult{}, errors.New(msg)
	}
	res := core.TextResult{Text: r.Result, CostUSD: r.TotalCostUSD}
	if len(r.StructuredOutput) > 0 && string(r.StructuredOutput) != "null" {
		res.Structured = r.StructuredOutput
	}
	if r.Usage != nil {
		res.Usage = core.Usage{InputTokens: r.Usage.InputTokens, OutputTokens: r.Usage.OutputTokens,
			CacheRead: r.Usage.CacheRead, CacheWrite: r.Usage.CacheWrite}
	}
	return res, nil
}

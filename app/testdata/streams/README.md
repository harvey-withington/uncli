# Recorded CLI streams

Raw stdout from real CLI runs, one folder per provider and pinned version.
These are the contract the adapters are tested against: every file here must
parse without error, and unknown lines must come out as `EvUnknown`.

For each stream `<name>.jsonl` there may be:

- `<name>.in.jsonl`: the lines UNCLI wrote to stdin (turns, control requests
  and control responses), in order.
- `<name>.args.txt`: the CLI arguments after the binary.

Paths, the account email and the org id are scrubbed (newer recordings also empty the user's own slash commands and agents in the `initialize` answer) (`C:\uncli-spike`,
`user@example.com`, a zero uuid). Signatures and ids are left as recorded.

## claude/2.1.285

Recorded with Claude Code 2.1.285 (`stable` on 2026-10-01), mostly on Haiku 4.5.

| Fixture | What it shows |
| --- | --- |
| `single-turn` | One turn, stdin closed after it; full `system/init` and `result` |
| `multi-turn-partial` | Three turns on one process with `--include-partial-messages`; `init` repeats every turn; cost is cumulative |
| `interrupt-then-continue` | `interrupt` control request mid-answer, `result/error_during_execution`, then a normal turn |
| `resume-partial` | `--resume <id>` of `multi-turn-partial`; same session id, remembers context |
| `set-model-live` | `set_model` control request switches haiku to sonnet without a respawn |
| `session-id-tool-use` | `--session-id <uuid>` chosen by the caller; Write and Read tool calls with results |
| `slash-commands` | `/cost`, `/context`, `/compact`, `/model`, `/help`, `/clear` sent as text; `compact_boundary`, `conversation_reset` |
| `perm-host-no-handler` | Default `--permission-prompts host` with no handler: tool silently denied |
| `perm-prompts-none` | `--permission-prompts none`: automatic denial, `system/permission_denied` |
| `perm-mode-dontask` | `--permission-mode dontAsk`: denial by mode |
| `perm-accept-edits-bash-allowlist` | Code profile shape: `acceptEdits` + `Bash(git status:*)`; Write allowed, other Bash denied |
| `perm-stdio-allow` | `initialize` handshake, then `--permission-prompt-tool stdio` `can_use_tool` request answered allow |
| `perm-stdio-deny` | Same without `initialize`, answered deny |
| `set-permission-mode-live` | `set_permission_mode` switches `acceptEdits` to `default` on a live process: the first Write runs unasked, the second is asked; MCP tools are asked about (recorded 2026-10-04) |
| `mcp-status-tool-hints` | `mcp_status` sent mid-turn returns each MCP tool with its annotations (`readOnly`, `destructive`, `openWorld`, true ones only); read and destructive tool calls both asked (recorded 2026-10-04) |
| `subagent-background` | A sub-agent started in the background (`run_in_background`): `background_tasks_changed` and `task_started`, its own lines with `parent_tool_use_id`, `task_notification`; the turn ends (`result`) saying it launched the agent, then the CLI starts **a turn of its own** with no user message (`init`, `status: requesting`) and answers with the agent's result (recorded 2026-10-07) |
| `subagent-foreground` | A sub-agent the main agent waits for: one turn, the agent's lines with `parent_tool_use_id`, its hand-back as the Agent tool's result, one `result` (recorded 2026-10-07) |
| `edit-tool-result` | `acceptEdits` with Read and Edit allowed: an Edit's `tool_use_result` carries `structuredPatch` hunks (`newStart` plus context lines), from which the changed line is found; a Read's carries the file (recorded 2026-10-06) |
| `wsl-perm-stdio-allow` | The `linux-x64-musl` build in an UNCLI WSL distro (Alpine 3.24.2, user `uncli`), run through `wsl.exe -d <distro> --cd <dir> -- claude …` and signed in with `CLAUDE_CODE_OAUTH_TOKEN` passed through `WSLENV`: the same `initialize` handshake and `can_use_tool` request as `perm-stdio-allow`, with Linux paths; `initialize` reports the account only as `{"tokenSource":"CLAUDE_CODE_OAUTH_TOKEN"}` (recorded 2026-10-07, decision 0011) |
| `wsl-perm-stdio-deny` | Same, answered deny: nothing written, the answer is DENIED (recorded 2026-10-07) |
| `image-input` | Base64 PNG image content block on stream-json input |
| `append-system-prompt` | `--append-system-prompt` and `--disallowedTools Bash` |
| `chat-profile-minimal` | Chat profile flags: `--system-prompt`, `--tools`, `--strict-mcp-config`, `--setting-sources ""`, `--disable-slash-commands` |
| `system-prompt-snapshot-resume` | Resume with a different `--append-system-prompt`: the original prompt still applies |
| `error-bad-model` | Unknown model: `result` with `is_error`, assistant `error: model_not_found` |
| `error-not-logged-in` | No credentials: `result` with `is_error`, assistant `error: authentication_failed` |

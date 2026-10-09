# Recorded CLI streams

Raw stdout from real CLI runs, one folder per provider and pinned version.
These are the contract the adapters are tested against: every file here must
parse without error, and unknown lines must come out as `EvUnknown`.

For each stream `<name>.jsonl` there may be:

- `<name>.in.jsonl`: the lines UNCLI wrote to stdin (turns, control requests
  and control responses), in order.
- `<name>.args.txt`: the CLI arguments after the binary.
- `<name>.stderr.txt`: what it printed on stderr, where that matters.

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
| `resume-lost` | `--resume` of a conversation the CLI no longer has (its files removed, as when a container is rebuilt): `No conversation found with session ID: <id>` on stderr, one `result` with `error_during_execution`, `num_turns: 0`, then exit 1 (recorded 2026-10-08 in a WSL container) |
| `image-input` | Base64 PNG image content block on stream-json input |
| `append-system-prompt` | `--append-system-prompt` and `--disallowedTools Bash` |
| `chat-profile-minimal` | Chat profile flags: `--system-prompt`, `--tools`, `--strict-mcp-config`, `--setting-sources ""`, `--disable-slash-commands` |
| `system-prompt-snapshot-resume` | Resume with a different `--append-system-prompt`: the original prompt still applies |
| `error-bad-model` | Unknown model: `result` with `is_error`, assistant `error: model_not_found` |
| `error-not-logged-in` | No credentials: `result` with `is_error`, assistant `error: authentication_failed` |

## antigravity/1.3.1

Recorded with the Antigravity CLI (`agy`) 1.3.1 on Windows on 2026-10-09,
on Gemini 3.8 Flash (Low), with `--gemini_dir=C:\uncli-agy` (decision 0012).
Turns are `{"event":"user","message":{"content":"…"}}` lines; each is sent
after the previous turn's `result`. `<name>.hook.jsonl` holds what the
`PreToolUse` hook was sent for each tool call, one JSON object a line.

| Fixture | What it shows |
| --- | --- |
| `multi-turn` | Two turns on one process: `init` once (with the model when `--model` is given), `step_update` per step (`user_input`, `agent_response` with `text_delta`), each answer step's own `usage` when it's `DONE`, and `result` per turn with cumulative `usage` and `num_turns` |
| `resume` | `--conversation <id>` of `multi-turn`: remembers the number |
| `error-bad-model` | An unknown `--model`: one `result` with `status: ERROR` and the available models in `error`, the same on stderr, exit 1 |
| `tools-hook-allow` | `--dangerously-skip-permissions` with UNCLI's hook allowing: `view_file`, `write_to_file` and `run_command` (PowerShell) as `tool` steps (`ACTIVE`, then `DONE` with `output`); the hook gets each call's full arguments and a `stepIdx` equal to the step's `step_index` |
| `tools-hook-deny` | The hook denying a `run_command`: the step ends `ERROR` with `tool call denied by pre-tool hook: <reason>`, and the agent says so |
| `subagent` | `invoke_subagent`: a `subagent` step with `subagent_info`, the main agent's reply, a `system_message` step when the sub-agent's report comes back, and the final answer, all in one turn; the sub-agent's own tool calls reach the hook under its conversation id |

## grok/1.0.50

Recorded with Grok Build (`grok`) 1.0.50 on Windows on 2026-10-09, over ACP
(`grok agent --no-leader stdio`, decision 0014), with a private `GROK_HOME`.
Each `.jsonl` is everything grok printed; `.in.jsonl` is what the client
sent, as UNCLI does: no files or terminal offered, permission requests
answered with the agent's `allow-once` or `reject-once`, any other agent
request refused. Paths are `C:\uncli-spike` (the work folder),
`C:\uncli-grok` (GROK_HOME) and `C:\Users\user`. The commands and skills
grok lists (`available_commands_update`) are emptied, since they include
the user's own.

| Fixture | What it shows |
| --- | --- |
| `initialize-signed-out` | `initialize` answered without a sign-in (capabilities, `_meta.modelState`), then `session/new` refused: `Authentication required` |
| `tools-allow` | A plain answer, then a turn with a PowerShell command, a file read (not asked about), a write (a diff with `oldText: ""`) and another command, each asked and allowed; the first `tool_call` names the tool only in its title and `_meta["x.ai/tool"]`, its kind comes in the next update; each prompt's result has the turn's usage and cost in `_meta.usage` (cost in 10^-10 US dollars) |
| `tools-deny` | A command refused (`reject-once`): the tool fails with "User rejected the execution…" and the turn ends `cancelled` |
| `cancel` | `session/cancel` while it thinks: `stopReason: cancelled`, then the same process answers the next prompt |
| `resume` | `session/load` of the `tools-allow` session: the whole conversation replayed as updates before the load's answer, then a prompt answered from it |

`models-signed-in.txt` and `models-signed-out.txt` are what `grok models`
prints in each state.

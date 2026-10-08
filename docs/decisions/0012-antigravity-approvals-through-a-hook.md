# 0012. Antigravity: approvals through a hook, with the CLI's own checks off

Status: accepted
Date: 2026-10-09

## Context

The Antigravity CLI (`agy`, Google) is UNCLI's second provider (Harvey,
2026-10-08), and the first test of UNCLI as a harness for any AI CLI. It
has a headless mode close to the first CLI's: `--input-format stream-json
--output-format stream-json` keeps one process for many turns, and the
stream has `init`, `step_update` (text deltas, tool steps, sub-agents) and
`result` events. Google's staff confirm that a third-party app may run the
official binary as a child process over stdin and stdout with its own
cached sign-in, which is how UNCLI works (forum thread on the BRUV card).

What it lacks is a way to ask about a tool. Headless `agy` has no
permission protocol on stdin and stdout: a tool that needs approval is
quietly denied. Allow rules don't help, because print mode ignores
`permissions.allow` and `toolPermission` entirely (upstream issue #548,
open since 2026-07-06, reproduced here with 1.3.1). A `PreToolUse` hook's
`allow`, with or without `permissionOverrides`, is denied the same way. The
only way to let a headless `agy` run anything is
`--dangerously-skip-permissions`.

The project rules said never to use that flag. Harvey, 2026-10-08: "I
actually have no objection to skipping permissions -- that was just a
preference for safety interpreted by the llm that wrote the design. Hooks
should provide a robust alternative."

## Decision

UNCLI runs `agy` with `--dangerously-skip-permissions`, and a `PreToolUse`
hook of UNCLI's own decides **every** tool call through UNCLI's approval
flow (the same policy, safe list, judging, cards and unattended mode as
the first CLI).

- **UNCLI's own state folder.** `agy` is started with the hidden
  `--gemini_dir=<config>/agy` flag, so its conversations, logs and
  configuration live in UNCLI's folder, not `~/.gemini`. The hook is
  defined there (`<gemini_dir>/config/hooks.json`, matcher `*`), so
  nothing is written into the user's projects or their global Antigravity
  configuration, and the Antigravity IDE and app are unaffected. Sign-in
  still works: `agy` keeps its Google credentials in the OS keyring, shared
  with the Antigravity IDE.
- **The hook.** The hook's command is UNCLI's own executable in hook mode.
  It reads the tool call (JSON on stdin), sends it to UNCLI over localhost
  with the session's address and a random token from its environment
  (`agy` passes its environment to hooks), waits for the answer, and
  prints `{"decision":"allow"}` or `{"decision":"deny","reason":…}`.
  Without UNCLI's address and token it denies.
- **It fails closed.** With `--dangerously-skip-permissions`, every failure
  of the hook blocked the tool call: a crash, going past the hook's
  `timeout`, output that isn't JSON, `{}` with no decision, and `deny`. Only
  `allow` ran it. Sub-agents' tool calls go through the hook too, under the
  sub-agent's own conversation id. These are tested against the real CLI,
  one test per failure, so a CLI update that changes them fails the
  integration tests before the version is bumped.
- **Pinned and quiet.** UNCLI downloads its own pinned `agy` from the
  versioned release URL and checks its SHA-512 (the release manifest's own
  scheme), and sets `AGY_CLI_DISABLE_AUTO_UPDATE=true` so `agy` never
  updates itself under UNCLI.

The project rule now reads: profiles never bypass approvals; an adapter may
switch off its CLI's own checks only when UNCLI gates every tool call
itself and fails closed, with a test for each way that can fail.

## Consequences

- UNCLI's approvals work the same for both CLIs: the user sees the same
  cards, safe list and modes.
- The hook is the only gate. If `agy` stopped running hooks for some tool,
  that tool would run unasked. The fixtures and the real-CLI tests cover
  the tools seen so far (commands, file reads and writes, sub-agents); a new
  `agy` version is pinned only after they pass.
- `--gemini_dir` is undocumented. If it goes, the fallback is the user's
  `~/.gemini` with UNCLI's hook added under its own name, which would also
  apply to the IDE; that would need another decision.
- The user's own Antigravity customizations (skills, plugins, MCP servers in
  `~/.gemini/config`) aren't loaded in UNCLI's folder. Sharing them, as
  containers share the first CLI's, is later work.
- What `agy` can't do, UNCLI does another way, through capability flags:
  no live interrupt (the process is stopped and the conversation resumed),
  no live model switch (respawn with `--conversation`), no images or PDFs
  (non-text content blocks end the session), slash commands end a streaming
  session, and usage comes as tokens without cost.
- If Google fixes #548 or adds a permission protocol, moving to it is a
  larger change (the hook server becomes unnecessary). That bridge is
  crossed when it comes (Harvey, 2026-10-08).

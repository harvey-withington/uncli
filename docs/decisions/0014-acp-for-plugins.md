# 0014. Plugins can speak ACP, through a per-process driver

Status: accepted
Date: 2026-10-09

## Context

Grok Build (`grok`, xAI) is to be the first real provider plugin (Harvey,
2026-10-09). Its line-a-turn headless mode (`grok -p … --output-format
streaming-json`) runs one prompt per process and can only approve
everything (`--always-approve`), with no hook. Its other mode, `grok agent
stdio`, speaks ACP, the Agent Client Protocol: JSON-RPC 2.0 over stdio with
requests both ways. ACP has:
- a session (`session/new`, or `session/load` to resume, which replays the
  conversation);
- prompts (`session/prompt`, answered when the turn ends with a
  `stopReason`);
- streamed updates (`session/update`: message chunks, tool calls with a
  standard `kind`, file diffs);
- real permission requests (`session/request_permission`, answered with one
  of the agent's option ids);
- an interrupt (`session/cancel`).

ACP is a standard. Gemini CLI speaks it, and Codex and Claude Code do
through bridges.

The manifest engine (decision 0013) can't speak it:
- a turn needs the session id from an earlier response;
- requests from the agent must be answered by id;
- every output line maps to events without the adapter replying.

`core.Adapter` encodes turns statelessly, and its parser is a function of
output lines alone.

## Decision

- **An optional per-process driver in core.** An adapter that is a
  `core.DriverAdapter` gives each CLI process a `core.Driver`. It writes its
  own opening lines, reads every output line (returning events, and lines to
  write back), and encodes turns and controls with what it has learned. The
  session uses a driver when the adapter has one, and the parser and
  encoders as before when it doesn't. The built-in adapters don't have one,
  so their path is unchanged.
- **ACP is the manifest engine's second protocol.** A manifest with
  `protocol: acp` describes only what isn't standard:
  - how to install and start the CLI;
  - its environment and state folder;
  - how it signs in;
  - its tool kinds where they differ.

  The protocol's mapping is fixed, in Go:
  - message chunks are the answer;
  - tool calls are the trace, with ACP's kinds as UNCLI's action kinds and
    diffs as changed files;
  - a permission request is an approval card, answered with the agent's
    `allow_once` or `reject_once` option;
  - Stop is `session/cancel`;
  - resume is `session/load`, whose replay is ignored;
  - an agent request UNCLI doesn't serve is answered "method not found", so
    the agent never waits on it.
- **UNCLI offers the agent no files or terminal** (`fs` and `terminal`
  capabilities off). The agent uses its own tools in its own process, and
  UNCLI's approvals decide what runs.
- **A device-code sign-in kind.** UNCLI runs the CLI's device sign-in (for
  Grok, `login --device-auth` in UNCLI's own state folder), shows the link
  and code it prints, opens the link, and waits for it to finish. The code
  can be approved on any device.

## Consequences

- One Go implementation serves every ACP agent; a new one is a short
  manifest.
- The session gains a second path (driver or parser). Each place it writes
  to the CLI goes through one function that picks the right one, so the two
  can't diverge.
- ACP has no standard per-turn token usage; ACP providers' pages may show
  none.
- Some agents end the turn when a tool call is refused (Grok's
  `reject_once` gives `stopReason: cancelled`), where UNCLI's built-ins
  carry on without it.

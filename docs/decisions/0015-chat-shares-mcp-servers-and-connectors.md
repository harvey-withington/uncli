# 0015. Chat gets the user's MCP servers and connectors

Status: accepted; superseded in part by 0016 (what Chat shares is now its default, chosen per session)
Date: 2026-10-16

## Context

Chat is isolated (`isolated: true`): it reads none of the user's CLI
settings, skills or instructions, which keeps a turn to ~2.4k input tokens
instead of ~26k. Isolation also took away every MCP server, and Harvey keeps
his personal organisation on an MCP server (BRUV), so Chat couldn't reach it
(2026-10-16).

What the pinned CLI (2.1.285) does, measured from its `init` line:

- `--setting-sources ""` drops the user-scope servers in `~/.claude.json`
  but keeps the account's claude.ai connectors.
- `--strict-mcp-config` drops the connectors too, keeping only the servers
  given with `--mcp-config`.
- `--tools` limits the built-in tools only; MCP tools are never filtered by
  it.
- With all of a typical user's servers and connectors (71 tools), a Sonnet
  turn costs ~39k input tokens. With `ToolSearch` among the tools, the CLI
  sends tool descriptions only when a turn looks them up: ~4.1k. Haiku
  doesn't use tool search, so on Haiku the full cost remains.

## Decision

- Profiles get `mcp` and `connectors` (`shared` | `none`, default `none`),
  the same words containers use (decision 0011). They only matter for an
  isolated profile; others get everything through the user's settings.
- `mcp: shared` gives the CLI the user-scope servers from `~/.claude.json`
  as one `--mcp-config` JSON argument. `connectors: shared` leaves out
  `--strict-mcp-config`. The settings stay out either way.
- Chat shares both and adds `ToolSearch` to its tools.
- Only a session on this machine shares them. A session in a container gets
  what its container profile shares.
- Approvals are unchanged: an MCP tool call is judged and, when needed,
  prompted for like any other (decision 0009).

## Consequences

- Chat can use the user's own data sources, the way a desktop chat app can.
- A Chat turn costs ~4k tokens on Sonnet or Opus instead of ~2.4k, and
  much more on Haiku.
- The server definitions, including any auth headers, go on the CLI's
  command line. Only the same user (or an administrator) can read it, and
  they can read `~/.claude.json` already. UNCLI doesn't log the arguments.
- Project-scope servers aren't shared: Chat's folder is a fresh one with no
  project of its own.

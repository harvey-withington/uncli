# 0009. MCP tool judgements hold for a server version

Status: accepted
Date: 2026-10-07

## Context

An MCP tool whose server gives no annotations is something UNCLI can't
place, so the decision model judges it. The judgement was cached by the
tool's name, so a server that later changed what a tool does kept the old
verdict: a small safety gap.

The plan was to fingerprint each tool by its description and input schema
and judge it again when they change. The CLI doesn't give UNCLI either:
`mcp_status` (fixture `mcp-status-tool-hints`, CLI 2.1.285) reports each
tool only as `{name, annotations}`, plus each server's `serverInfo` (name
and version). `can_use_tool` carries no description either.

Three ways were weighed (BRUV card "Judge MCP tools per server"):

1. Key judgements on the server's name and version.
2. Have UNCLI connect to each server itself (`tools/list`) to read
   descriptions and schemas.
3. Ask for a richer `mcp_status` upstream and wait.

## Decision

Option 1 (Harvey, 2026-10-07).
- **Version in the key:** a judgement of an MCP tool is cached under
  `mcp:<tool>@<server>/<version>`, so a server update judges its tools
  again.
- **One decision per server:** when the CLI reports its tools, every
  unannotated tool of a server not yet judged for that version is judged in
  one decision, at most 20 tools per round. Seeing the tools side by side
  helps, since names are all the decider has.
- **Fallback:** if the batch fails, each tool is judged on its own when it
  is used, as before.

Option 2 is ruled out: UNCLI talks only to its CLI, and its own connection to
each server would duplicate the CLI's (credentials, transports, lifetimes).

## Consequences

- A server that changes a tool without changing its version keeps the old
  judgement until it does.
- Judgements from before this change were keyed by name alone, so each tool
  is judged once more, now with its version.
- The user's own safe-list entries for a tool aren't tied to a version:
  they are the user's word and stand across updates.

# 0002. Approvals travel over stdio, not a local MCP server

Status: accepted
Date: 2026-10-01

## Context

The brief planned a local MCP server on 127.0.0.1 for phase 2 approvals: a
per-session token, an `approve` tool named by `--permission-prompt-tool`, and
Origin and content-type checks to keep web pages out. The spike found that
Claude Code 2.1.285 accepts `--permission-prompt-tool stdio`. It then writes a
`control_request` with subtype `can_use_tool` to stdout and waits for a
`control_response` (`behavior: allow` with `updatedInput`, or `behavior: deny`
with a message) on stdin, which is the same channel UNCLI already uses for
turns, interrupts and model switches.

## Decision

Phase 2 approvals use the stdio control protocol. The Claude adapter's parser
emits `EvApprovalAsked` for `can_use_tool`, and `EncodeControl` writes the
answer. There is no `permission/` package and no local server. Phase 1 passes
`--permission-prompts none` so prompts are denied immediately instead of
waiting on an answer.

## Consequences

- No listening port, token or Origin checks: one less attack surface and one
  less component.
- Approvals work unchanged in a container (phase 5), because stdio is already
  the container transport.
- Approvals depend on the CLI's control protocol, which is less documented
  than MCP; fixtures `perm-stdio-allow` and `perm-stdio-deny` pin its shape.

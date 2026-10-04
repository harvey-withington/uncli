# 0003. UNCLI applies the profile allowlist, not the CLI

Status: accepted
Date: 2026-10-04

## Context

Session modes (Read-only, Ask, Full) decide UNCLI's answer to each
`can_use_tool` request. But the CLI only asks about what it wouldn't run
itself: anything in `--allowedTools` and, under `acceptEdits`, file edits
(and `mkdir`, `mv`, `rm`) inside the folder run without UNCLI ever seeing
them. In Read-only mode the Code profile's allowlist (`npm test`, `go build`)
and its accepted edits would still run. Project deny rules had the same blind
spot: a rule denying `npm` never fired, because the allowlist approved
`npm test` first.

The CLI can't change its allowlist on a running process, but it can change
its permission mode live (`set_permission_mode`, fixture
`set-permission-mode-live`).

## Decision

When the CLI routes its prompts to UNCLI (`Approvals`), UNCLI does not pass the
profile's `allowed_tools` to the CLI. It parses them (same syntax) and applies
them itself, in Ask mode, after the session mode and the project's rules. The
CLI runs in the profile's own permission mode, except in Read-only, where
UNCLI switches it to `default` (live, or by respawn for an adapter without
`LivePermissionMode`) so that it asks about edits too.

## Consequences

- Read-only really is read-only, and a project's deny rules beat the
  profile's allowlist.
- Every tool use outside the CLI's own read-only set now makes a round trip
  to UNCLI, answered at once for allowlisted tools; no visible cost.
- UNCLI interprets the CLI's allowlist syntax itself (tools, command prefixes,
  file globs); qualified forms it doesn't understand, such as
  `WebFetch(domain:…)`, never match and fall back to asking.
- Without `Approvals` (a CLI that can't route prompts), the allowlist still
  goes to the CLI as before, and session modes can't apply.

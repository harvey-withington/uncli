# 0001. UNCLI installs and pins its own CLI

Status: accepted
Date: 2026-10-01

## Context

UNCLI parses the Claude CLI's stream-json output, which changes between CLI
releases. The CLI updates itself, so a user's install can move under UNCLI at
any time. The spike machine also had no `claude` on PATH at all: the only copy
was bundled inside the VS Code extension, at a path that changes with every
extension update. UNCLI's users are, by design, not CLI people.

## Decision

Each UNCLI release pins one CLI version per provider (Claude Code 2.1.285 to
start). UNCLI downloads that version itself from the official distribution
(`downloads.claude.ai/claude-code-releases/<version>/`), checks its SHA-256
against the release manifest, keeps it in UNCLI's own cache directory, and
runs it with the CLI's auto-updater disabled. The user's own install is never
used or modified. A setting lets the user run a different version (`stable`,
`latest` or a specific one) at their own risk, and return to the pinned one.

## Consequences

- A CLI update can't break UNCLI; fixtures are recorded per CLI version and
  tested against the pinned one.
- First run downloads about 240 MB, so it needs a progress screen.
- UNCLI must handle sign-in itself (`claude auth status` / `claude auth login`).
- Moving to a new CLI version is a deliberate UNCLI change: record fixtures,
  run the parser tests, bump the pin.
- UNCLI still only runs the official, Anthropic-signed binary, so subscription
  terms are respected.

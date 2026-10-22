# 0018. Slash commands are offered from the CLI, and guarded

Status: accepted
Date: 2026-10-23

## Context

UNCLI passed any message starting with `/` to the CLI as typed, and the
toolbar had fixed `/compact` and `/context` buttons. There was no list of
the commands a CLI takes and no completion (Harvey, 2026-10-23). The CLIs
do report them:

- **Claude Code 2.1.285:** the handshake reply lists `commands` with name,
  description, argument hint and whether it's built in (54 in the fixture).
  The init line names `terminal_slash_commands`, four that only work in its
  own terminal (`doctor`, `color`, `focus`, `reload-plugins`).
- **ACP agents:** the standard `available_commands_update` gives name,
  description and an input hint. Grok 1.0.50 sends its list in the
  `initialize` reply's `_meta.availableCommands` and then empty updates
  whose `_meta` still counts them.
- **Antigravity CLI:** takes no slash commands. One sent as text ends a
  streaming session.

Two gaps came up along the way. `Capabilities.SlashPassthrough` was
declared but never checked, so a command went to any CLI, Antigravity
included. And Grok has `/always-approve`, which "skips all permission
prompts": typed into a session, it would stop Grok asking UNCLI before it
uses tools, which the rule that UNCLI gates every tool call forbids.

## Decision

- Adapters report a session's commands as `core.EvCommands` (name,
  description, hint), Claude's less its terminal-only ones. An empty ACP
  list keeps the last one. The manager keeps each CLI's latest list (in the
  settings store), so a session that hasn't started its CLI yet still has
  one to offer.
- **Guard:** a command whose name could switch off prompting (matching
  approve, bypass, yolo, skip-perm, dangerous, auto-mode or trust) is
  neither offered nor sent. Sending one is refused with the reason. A CLI
  without `SlashPassthrough` gets no command at all, and ACP agents now
  declare it. The match is by name because no CLI says which commands
  change permissions; it errs towards refusing.
- **UI:** typing `/` at the start of the message box opens `SlashMenu` with
  the matching commands. The command palette lists them as "Run /name".
  The toolbar's slash buttons show only for CLIs that take commands.

## Consequences

- Skills, plugin commands and the user's own commands can be found
  without knowing their names.
- A harmless command whose name happens to match the guard (say,
  `/trust-report`) can't be sent from UNCLI. If that turns out to matter,
  the fix is a per-provider allowance in its manifest, not a looser
  pattern.
- A message that merely starts with a path (`/usr/bin …`) isn't taken for
  a command.

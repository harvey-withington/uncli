# 0004. Ask mode asks about risk, not about commands

Status: accepted
Date: 2026-10-04

## Context

Ask mode first asked about any tool use that no rule and no entry in the
session type's allowlist covered. That made the user responsible for
knowing which commands are safe: Claude routinely runs chained PowerShell
lines (`git status --short; npm --prefix "<folder>" test; npm run lint`), and
each new form either asked or needed a rule the user had to write. UNCLI is
for people who don't use the command line; Harvey's verdict was that "the
product is failing if it requires the users to know every CLI command the
LLM might want to run."

## Decision

Ask mode judges each tool use, or each working part of a command, by what
it could do (`session/risk.go`): it **looks**, it is **routine** work inside
the session's folder (edits, builds, tests, formatters, package scripts,
local installs, local git), or it is **risky** (deletes for good, throws
away uncommitted work, changes files outside the folder, sends or
publishes, installs software, changes the system, stops programs, connects
elsewhere, touches secrets, runs code it was handed, changes a cloud
account). Looking and routine work run; risky work asks, with a card that
says why in plain words before showing the command.

Recognition comes from tables of programs and subcommands, PowerShell verbs
(Get- looks, Set- is routine, Remove- is risky) and a check of the paths a
command names, not from per-user rules. For a command it doesn't recognise,
the user chooses (a preference): the quick-task model judges it (the
default; once per command, cached in `risk_judgements`, run through the CLI
like any quick task), it runs if it stays in the folder, or the user is
asked. Rules and the allowlist stay as optional overrides.

## Consequences

- Most real work runs without a card, and cards are rare enough to read.
- The tables decide what is routine, so a mistake in them lets something
  run unasked. They err towards risky: anything outside the folder, any
  recursive or wildcard delete, any push or publish asks; unrecognised
  programs never count as routine unless the user chose that.
- The model's judgement costs a little usage and a few seconds the first
  time each unrecognised command is seen; the request waits silently. If the
  model fails, the user is asked.
- Quick tasks now run in an empty folder of their own, so a judgement (or a
  summary) never reads the project's `CLAUDE.md` or memory.

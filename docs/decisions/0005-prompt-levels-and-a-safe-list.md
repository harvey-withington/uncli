# 0005. Prompt levels and a safe list that learns by class

Status: accepted
Date: 2026-10-04
Refines: 0004

## Context

After 0004, the session header offered three modes, Read-only, Ask and
Full, plus a globe switch for "tools that reach outside this computer".
Harvey's review found them confusing. The labels weren't analogs of each
other: two named what could run and one named a behaviour. Read-only
refused to run tests, which change nothing. The outside switch was jargon
nobody asked for. Corrections lived in three places (project rules,
session rules and per-tool classes), keyed on command prefixes the user
had to know.

The bar for this design: if prompting doesn't learn properly, people turn
it off. That is what happens with Claude Code today, where users run with
permissions bypassed because it prompts too often and forgets.

## Decision

- **The control says when to prompt:** "Prompt me: Always / When unsafe /
  Never". Reading runs at every level. Always prompts for everything else.
  When unsafe (the default) prompts only for unsafe work, and the user's
  setting decides what UNCLI can't place. Never runs everything except
  what the user blocked. Questions and plans always reach the user.
- **Unattended is a separate switch:** never prompt, but decline whatever
  would have prompted. Always plus Unattended replaces Read-only. The
  outside switch is dropped. Sending or publishing is one of the unsafe
  reasons, and reading the web counts as reading.
- **One safe list** replaces rules, session rules and tool classes. Each
  entry is a class (a command class or a tool), a verdict (Safe, Unsafe or
  Blocked) and a scope (This project or All projects). The same scope
  control appears on cards, in the trace and in Settings. The most
  specific entry wins, and This project beats All projects on a tie.
- **What one click learns is the command's class**, not the command. The
  class is the program plus its subcommand (`npm run` keeps the script)
  plus the flags that change its risk (`--force`, `--hard`, `--global`,
  `-r -f`). Files, messages and other options are dropped. Deletes, kills
  and remote logins keep their targets. The card names the class before
  it is saved. Nothing is learned when the risk comes from context (a
  path outside the folder, secrets), from inline code, or from a target
  only known at run time. Those always prompt, and the card says why.
- **Learning goes both ways:** "This is safe" on a card, and "This should
  prompt" on a trace row that ran because UNCLI judged it safe.
- **The session judges provider-neutral actions** (`core.ToolAction`).
  The adapter maps its tools to kinds, a shell dialect and paths, so
  another CLI, a container runtime or a profile only adds a mapping. The
  dialect comes from the tool use, not the host OS, so the reader works
  the same on macOS and Linux, where Claude uses bash.

## Consequences

- Prompts are measured, not guessed. A replay of 7,587 real tool uses
  (`TestReplayPrompts`) gives these prompt rates for When unsafe: 18.6%
  when what UNCLI can't place prompts, and 3.5% when it is treated as safe
  inside the folder. Teaching each class once cuts those to about 12% and
  2%. The default lets the quick-task model judge, which sits between the
  two. A committed corpus (`TestRiskCorpus`, about 170 commands) pins the
  tables.
- A class is coarser than a command. Marking `npm run test` safe covers it
  with any files and options, but never `npm run deploy` and never a
  risky-flag variant.
- The class is shown in the user's words rather than as a pattern. A
  wrong lesson is one click away in Settings: change the verdict or the
  scope, or remove the entry.
- Old rules and classes migrate (allow → Safe, ask → Unsafe, deny →
  Blocked), and old session modes map: readonly → always, ask → unsafe,
  full → never. Session-only rules are gone; a scope of This project
  replaces them.

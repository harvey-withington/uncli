# 0016. Each isolated session chooses the user's tools and skills

Status: accepted (supersedes 0015's "Chat shares both", which becomes Chat's default)
Date: 2026-10-16

## Context

Decision 0015 gave Chat the user's MCP servers and connectors. Harvey then
asked for their skills too, and for a way to start a session without either
(2026-10-16). Both are launch flags: changing them restarts the CLI. The
tool list is part of what the provider caches, so the next turn re-reads the
conversation in full.

What the pinned CLI (2.1.285) does with skills, from its `init` line:

- `--disable-slash-commands`, which Chat used, also turns skills off.
- With no setting sources the user's skills don't load, and loading the
  `user` source would bring their settings (hooks, permissions,
  environment, plugins) too.
- A project folder's `.claude/skills` loads with `--setting-sources
  project`. But a Chat session can write in its own folder, so a hook
  written to `.claude/settings.json` there would run.
- `--plugin-dir` loads a plugin with every setting source off. Only skills
  inside the plugin's folder load: a `plugin.json` naming the user's skill
  folders loads nothing, so the skills must be linked in. The plugin's name
  goes before each skill's (`my:docx`).
- With skills on, the CLI adds 18 of its own (mostly for coding).
  `CLAUDE_CODE_DISABLE_BUNDLED_SKILLS=1` leaves two (`design`, `doctor`).
- Cost of a Sonnet turn, measured: ~1.9k tokens with nothing, ~5.9k with
  12 skills (plus `Read` and `Skill`), ~4.1k with 71 MCP tools behind
  `ToolSearch`.

## Decision

- A session can share two things of the user's own: **tools** (their MCP
  servers and connectors, one choice in the UI) and **skills**. A profile
  sets the defaults with `mcp`, `connectors` and `skills`. When tools are
  on, the session gets what the profile shares of the servers and
  connectors, or both if it shares neither. Chat defaults to all three.
- **Chosen when the session is created**, in the new-session dialog (each
  type remembers its last choice). **Changed from the header**, in a menu
  behind a quiet note after the folder. The choice is stored on the
  session (`sessions.shares`; NULL means the type's defaults). A change
  applies from the next turn, restarting the CLI on the conversation the
  same way an effort change does. It isn't a toolbar toggle: it's rarely
  changed and costs a full re-read of the conversation.
- Only an isolated type has the choice; the others read the user's settings
  anyway. It's also offered only on this machine (a container shares what
  its container profile does) and only on a CLI that can (capability flags
  `SharedTools` and `SharedSkills`).
- **Skills come in through a plugin folder of UNCLI's own**
  (`<cache>/uncli/skills/claude`). It holds directory junctions (symlinks
  elsewhere) to the user's skill folders, including the ones synced from
  claude.ai. The links are checked each time the CLI starts. Only links are
  ever removed, never what they point to or anything that isn't a link.
  The CLI's own skills are turned off.
- Tools are added for what is shared: `Skill` and `Read` with skills,
  `ToolSearch` with tools. The adapter adds them, so a user's own profile
  gets them too. `Read` is judged like any read (`looks`, except secrets).
  `Bash` is not added, so skills that run scripts can't run them in Chat.

## Consequences

- A chat can use BRUV and the user's writing and document skills, or start
  clean when that's wanted.
- A chat with both costs ~8k tokens a turn on Sonnet instead of ~2k. Haiku
  doesn't use tool search, so it pays for every MCP tool on every turn.
- Skills that need to run scripts (`docx`, `pdf`, `xlsx`) only partly work
  in Chat. Giving Chat a shell is a separate decision.
- The plugin's links point into the user's CLI config folder. Editing a
  skill there shows up at the next start with nothing to sync.

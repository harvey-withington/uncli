# 0023. A repository's .uncli folder holds its team's UNCLI setup

Status: accepted
Date: 2026-10-26

## Context

A team that works in one repository wants everyone's sessions there to run
the same way: in the same container, under the same rules. Today each person
builds that by hand in Settings → Containers, and the result lives in their
own config folder (`containers.yaml`, decision 0011), where nobody else can
see it.

Other tools solve this with a folder in the repository: `.vscode/` for the
editor, `.devcontainer/` for a development container. Harvey (2026-10-26)
wants UNCLI's to replace `.devcontainer` for teams willing to run UNCLI, not
to stay compatible with it. UNCLI's containers are its own WSL distros, not
OCI images (decision 0011), and the aim is a better way to do it, not a
second reader of the old one.

UNCLI is to run as a desktop app, a VS Code extension (wanted first) and a
phone client, all clients of one service per user (brief, "Server mode and
embedding"). The phone is for day-to-day prompting; setup and configuration
stay off it (Harvey, 2026-10-26).

A repository's files are written by whoever can push to it. A cloned
repository is not trusted: a container's setup steps run as root with the
network on when it's built, and safe-list entries and folder mounts loosen
checks. The CLI already reads its own files from the repository (`.claude/`,
`CLAUDE.md`, `.mcp.json`, skills, agents); UNCLI must stay provider-neutral
and not duplicate them.

## Decision

- **The folder.** `.uncli/` at the root of a repository. The backend (not
  the UI) looks for it from a session's folder up to the repository root
  when the session starts, so every client gets the same result. Its files
  carry `version: 1`.
- **Layering.** Built-ins, then the repository's `.uncli/`, then the user's
  config folder, matched by `id` as UNCLI's YAML is today. Restrictions are
  the exception: the strictest wins.
- **What it holds, first:**
  - `containers.yaml`: container profiles in today's format (base, packages,
    setup steps, shares).
  - The default container for Code sessions in the repository.
  - Team policy (restrictions): a lowest prompt level allowed, no
    Unattended.
- **Later:** safe-list entries for the project's own commands; toolbar
  buttons and modifiers; required secrets by name only (checked against
  `~/.secrets`, decision 0017); folders inside the repository to mount, by
  relative path only.
- **What it never holds:** anything the CLI reads from the repository
  itself; secrets; personal settings (theme, layout, sign-ins); Windows
  folders outside the repository.
- **Trust.**
  - Restrictions apply without asking: they only make sessions safer.
  - Everything that runs code or loosens a check (building a container,
    safe-list entries, mounts) waits for the user. The first time UNCLI
    sees the folder, and whenever its files change, it shows what they
    would do and asks.
  - The answer is kept per repository with the files' checksum, so any
    edit asks again.
  - Untrusted means ignored, not blocked: sessions run without the
    repository's settings.
- **Where it's managed.** Trusting a repository and building its container
  happen in the desktop app and the VS Code extension only. The phone uses
  the result (its sessions run in the team's container under the team's
  policy) and can't trust, change or build anything.
- **No `.devcontainer` reading.** UNCLI doesn't read, convert or keep in step
  with `.devcontainer`.

## Consequences

- A team commits its UNCLI setup once, and a new member gets it on first
  trust: no Settings walk-through, and changes arrive with `git pull` (shown
  as "Changed since it was built" and asked about again).
- The format is a public contract: other repositories depend on it, so a
  change is a breaking change for them, as with the `--uncli-*` theme tokens
  (decision 0010).
- Repositories that also use `.devcontainer` keep two definitions until they
  drop one; UNCLI doesn't help with that, by choice.
- Containers exist only on Windows (WSL) until the Linux and macOS runtimes
  are built, so a repository's container does nothing there yet; its policy
  still applies.
- The trust prompt is a new kind of question to the user, and needs a design
  of its own (what it lists, in what words) before this is built.

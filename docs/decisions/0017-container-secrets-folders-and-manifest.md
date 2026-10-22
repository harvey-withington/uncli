# 0017. Containers mount a secrets folder, folders of their own, and list their mounts

Status: accepted
Date: 2026-10-22

## Context

A working session in the Sandbox container (2026-10-22) hit four gaps,
written up as "secrets share, custom folder mounts, artifacts and skill
sharing for UNCLI containers":

1. A Bitbucket token for `git fetch` had no supported way in, so it went
   into a folder UNCLI mounts for something else, which ties a secret to
   UNCLI's internal layout.
2. There was no way to mount another Windows folder when a need came up.
3. The artifacts panel showed nothing.
4. A skill linking to `../_shared/<doc>.md` lost the file, because skill
   folders are mounted one by one.

It also reported a subagent's MCP calls failing with "There is no turn to
approve this for".

What was found:

- Item 3 is a session type, not the container. Chat and Co-work keep
  `./artifacts/` in the session folder, which is the container's mounted
  workspace, so their artifacts already reach the panel. The session was a
  Code session, which has no artifacts panel anywhere: an `artifacts/`
  folder would add files to the user's repository.
- "There is no turn to approve this for" is UNCLI's own refusal of a
  permission request that comes when no turn is open, which is what a
  subagent left running in the background does after its turn ends.
- WSL's drive mounts (drvfs over 9p) ignore `-o ro`: the mount comes out
  read-write and the CLI's user can write to it. `mount -o remount,ro`
  afterwards works (tested on uncli-sandbox), and only root can undo it,
  which sessions can't become.

## Decision

- **Secrets:** a fourth share, `secrets: shared` (opt-in, off by default
  and in the built-ins). `%USERPROFILE%\.secrets` is mounted read-only at
  `/home/uncli/.secrets`, and `UNCLI_SECRETS_DIR` is set. The folder is the
  user's to fill, one file per service, and UNCLI never reads it. A missing
  folder is skipped, and Settings lists it.
- **Folders:** a profile lists more folders (`folders:` with `source`,
  `name` and `access`), each mounted at `/home/uncli/<name>`, read-only
  unless `access: rw`. Only `%USERPROFILE%`, `%APPDATA%` and
  `%LOCALAPPDATA%` expand, so a profile names no user. Refused: a whole
  drive; anything that is or contains the user profile, AppData,
  `%APPDATA%`, `%LOCALAPPDATA%`, Windows, Program Files or ProgramData; and
  anything in or containing a folder that holds sign-ins (the CLI's config
  folder, UNCLI's own data). Names are lower-case and can't start with a
  dot, so they can't land on `.claude` or `.secrets`. There are no
  arbitrary Linux targets.
- **Read-only** is a remount after the mount, in the one mount script that
  every UNCLI mount goes through. `wsl.conf` keeps automount, fstab and
  interop off.
- **Manifest:** the runtime records every mount it makes (name, Windows
  source, target, access, kind: builtin, share or custom), forgets them
  when the distro may have stopped, and writes them to
  `/etc/uncli/mounts.json` at each session start. The file is root-owned
  and mode 644, so sessions can read it but not change it, and
  `UNCLI_MOUNTS` names it. A share or folder that's turned off is
  unmounted at the next start.
- **The model is told:** a session in a container gets a directive, sent
  once and again when it changes, that it runs in a container, where the
  manifest is and, when shared, where the secrets are, with "never print
  it".
- **Skill support folders:** the per-skill mounts stay (the synced
  claude.ai skills need them). Folders beside the user's own skills that
  aren't skills (no `SKILL.md`, not hidden, not `synced`), such as
  `_shared`, are mounted beside them, so relative links resolve as on the
  host. Synced skills come whole. New skill folders appear from the next
  session start.
- **Artifacts:** session types with artifacts get `UNCLI_ARTIFACTS_DIR`
  (in a container, at the path the CLI sees). Code is unchanged.
- **Background requests:** a permission request with no open turn, while
  the CLI reports background tasks, opens the page of the turn the CLI
  carries on with (as that turn's start does) and is judged there.
  Anything else outside a turn is still refused.

## Consequences

- Tokens and API keys have one place, `~/.secrets`, the same path a Podman
  dev container mounting `%USERPROFILE%\.secrets` uses, so skills and docs
  can name it.
- Mounts are honest about access: read-only means the session can't
  write.
- A mount costs one `wsl.exe` call per session start (as before), plus one
  for the manifest.
- Folders are refused rather than warned about. If someone needs one of
  the refused folders, that's a later decision about an override.
- Code sessions still have no artifacts panel.

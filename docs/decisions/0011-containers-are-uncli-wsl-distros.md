# 0011. Containers are UNCLI's own WSL distros

Status: accepted
Date: 2026-10-07

## Context

Phase 5 of the brief runs the CLI in a container through the Docker Engine
API (`docker exec -i <ctr> claude …`), with container profiles compiled into
a cached Dockerfile layer. That makes Docker a prerequisite. Docker Desktop
is a large install of its own, needs a paid licence in bigger companies, and
puts a second product between UNCLI and the CLI. UNCLI is meant to stand
alone.

Every way of running Linux on Windows rests on Windows' own virtualisation,
so some one-time setup can't be avoided. WSL2 is that layer, ships with
Windows, and can be driven from UNCLI without anything else installed. These
were weighed (BRUV card "Phase 5: containers as UNCLI's own WSL distros"):

1. Docker or Podman as a prerequisite (the brief's design).
2. UNCLI's own WSL distros, with no container engine.
3. Podman inside UNCLI's distro, for real OCI images.
4. Windows Sandbox (Pro only, wiped on every close) or a VM of UNCLI's own
   through Hyper-V's APIs (a lot to build).

## Decision

Option 2, and only option 2 (Harvey, 2026-10-07: "100% WSL distro"). No
Docker, no Podman, no container engine, on Windows.

- **A container is a WSL distro UNCLI owns.** Each container profile
  becomes a distro named `uncli-<profile id>`, imported with
  `wsl --import` into UNCLI's data folder from a pinned, checksummed root
  filesystem. UNCLI never touches distros it didn't create.
- **The process is unchanged.** The runtime starts
  `wsl.exe -d uncli-<id> --cd <dir> --user uncli -- claude -p …` and talks to
  it over stdin and stdout exactly as the local runtime does, so the
  adapter, approvals over stdio and fixtures carry over.
- **The CLI is the Linux build of the pinned version.** It is fetched from
  the same release manifest and checked against its SHA-256, like the
  Windows build. The first base is Alpine with the `linux-x64-musl` build,
  which keeps the root filesystem a few megabytes. A profile can name
  another pinned base (Debian with `linux-x64`) if musl gets in a toolchain's
  way.
- **Profiles are built, not pulled.** `containers.yaml` stays config over
  code: base, packages, toolchains and MCP servers. UNCLI builds a profile by
  importing the base and running its install steps once. It keeps the result
  as an exported tarball, so a rebuild or a new copy doesn't start from
  scratch.
- **Locked down by configuration.** Each distro's `/etc/wsl.conf` turns off
  automounting Windows drives and Windows interop (no Windows programs from
  inside). Only the session's folder is mounted. The CLI runs as an
  unprivileged `uncli` user, never root. Credentials are never mounted; the
  OAuth token from `claude setup-token` goes in the Windows credential store
  and reaches the CLI as an environment variable.
- **Brain modes map onto distros.** Sandboxed is the distro's own `~/.claude`,
  which persists with it. Shared mounts the user's skills, commands, agents
  and memory folder, with `settings.json` generated read-only. Reviewed is a
  later step. Transcripts stay in the distro, where the transcript reader
  finds them through its location (`\\wsl.localhost\uncli-<id>\…`).
- **Turning WSL on is UNCLI's job.** If WSL is missing, UNCLI asks once for
  admin rights to run `wsl --install --no-distribution` and says when a
  restart is needed. Without WSL, local sessions work as before and container
  profiles say what's missing.
- **Other platforms** get the same runtime seam later: Linux through its own
  namespaces (no VM), macOS through Apple's Virtualization framework. Both
  are deferred.

## Consequences

- No Docker images or Dockerfiles: profiles aren't OCI images and can't be
  shared through a registry. A profile is reproducible from its YAML and the
  pinned base instead.
- All WSL distros share one lightweight VM and kernel, so profiles are kept
  apart from each other less strongly than separate VMs would be. The
  boundary between the CLI and Windows is still a VM boundary.
- Per-profile network policy can't use the VM's network, which the distros
  share. An allowlist will be firewall rules inside each distro, which hold
  because the CLI isn't root there.
- Reading Windows folders from WSL is slow for large trees. The phase 5
  spike measures it; if it's too slow, a Code session keeps its working copy
  inside the distro and changed files, IDE links and snapshots map paths
  back.
- The brief's Docker design in phase 5 (`docker exec`, the Docker Engine
  API, Dockerfile layers) is superseded by this record. The brief itself
  stays frozen.

## Spike results (2026-10-07)

On WSL 3.0.1 with Alpine 3.24.2 and CLI 2.1.285 (`linux-x64-musl`, which
needs `libgcc` and `libstdc++`):

- **Working copy inside the distro for Code sessions.** On this repo,
  `git status` takes 5.3 s cold on a mounted Windows folder against 28 ms on
  the distro's own disk, and writes are about 150 times slower. Changed
  files, IDE links and snapshots map paths back to Windows. Chat and
  Co-work folders stay mounted (root runs
  `mount -t drvfs '<folder>' <path> -o metadata,uid=1000,gid=1000` in the
  same `wsl.exe` call that starts the CLI, since mounts end when the distro
  stops).
- **Interop needs a boot command too.** `[interop] enabled=false` leaves
  the `WSLInterop` binfmt handler registered, so a Windows program copied
  into the distro still tries to start. `[boot] command = "echo 0 >
  /proc/sys/fs/binfmt_misc/WSLInterop"` turns it off.
- **The first imported distro becomes the user's default.** UNCLI puts
  the default back.
- **Sign-in works without a browser on the machine.** `claude setup-token`
  in a pseudo-terminal prints a link and takes the code the sign-in page
  shows, so UNCLI's screen shows the link, takes the code and saves the
  token to the credential store. It reaches the CLI through `WSLENV`, which
  passes only the variables it lists.
- **The protocol is unchanged.** Turns and approvals over `wsl.exe` stdio
  match the Windows fixtures, with Linux paths (fixtures
  `wsl-perm-stdio-allow` and `wsl-perm-stdio-deny`). `wsl.exe` prints its own
  errors on stdout, so UNCLI sets `WSL_UTF8=1` and treats a first line that
  isn't JSON as a runtime error. With a token, the CLI doesn't report the
  account's email or plan.

## Built (2026-10-08)

- **bash is in every container.** The CLI's shell tool refuses to run
  without it ("No suitable shell found"), so in a container with only
  BusyBox's shell every command failed, and Claude reported git as missing.
- **Shared brain** (`brain: shared`) mounts, for each session, the project's
  memory folder from Windows at the name the container's path gives it
  (`~/.claude/projects/<project dir>/memory`), so what Claude remembers there
  is kept on Windows. It also mounts the user's skills, agents and commands,
  copies in their CLAUDE.md, and writes their git name and email to the
  container's `.gitconfig`. Their settings and credentials stay out.
- **Shared MCP servers** (`mcp: shared`) copy into the container's
  `~/.claude.json` the user's MCP servers (user scope, and the project's)
  that can run on Linux: URL-based ones, and `npx`, `node`, `uvx`…
  commands (a Windows `cmd /c npx …` is unwrapped) whose arguments name no
  Windows path. An HTTP server reached over Tailscale connected from WSL.
- The built-in **Sandbox** shares both, with the usual tools (GNU coreutils,
  curl, jq, ripgrep, make, Node and Python). **Isolated** shares nothing.
- **The CLI waits for shared MCP servers.** By default it connects MCP
  servers in the background, so a server still connecting when the first
  message goes (a few seconds from a container) has no tools in that
  message: the model answers that it has none. Containers that share MCP
  servers set `MCP_CONNECTION_NONBLOCKING=0` and `MCP_CONNECT_TIMEOUT_MS=15000`,
  so the first message has them; a server that never answers delays the
  start by 15 s at most.
- **Conversations outlive a rebuild.** The CLI keeps its conversations in
  `~/.claude/projects`, which was inside the distro, so rebuilding a
  container lost them and its sessions couldn't resume. That folder is now
  UNCLI's on Windows (`<config>/wsl-data/<container>/projects`), mounted on
  each start, with a shared brain's memory folder mounted inside it.
- **A lost conversation doesn't strand a session.** A CLI asked to resume a
  conversation it no longer has fails at once (fixture `resume-lost`), and
  would on every message after. The adapter recognises it, the session
  forgets the conversation, the page says so, and the next message starts a
  new one; this covers a conversation deleted on Windows too.
- **A container says when it's out of date.** A record of what it was built
  from (UNCLI's setup fingerprint, the base and its checksum, the CLI
  version, the packages) is kept with its conversations
  (`wsl-data/<container>/build.json`). Settings compares it with how the
  container would be built now and says what changed; containers.yaml is
  re-read each time, and what a container shares applies from its sessions'
  next start, so only what's baked in needs a rebuild.
- **claude.ai skills reach containers as ordinary skills.** The CLI keeps
  them in `skills/synced/<org>_<account>/` and loads only the folder of the
  account it's signed in to; signed in with a token it doesn't know the
  account, so it loaded none. A shared brain now mounts each skill (the
  user's own, then the synced ones) one by one into the container's own
  skills folder. claude.ai connectors (MCP servers that come with the
  account) still need the account sign-in and aren't available in
  containers.
- **claude.ai connectors with the whole-account sign-in** (Harvey: Jira,
  Office). The models-only token can't fetch them; `claude auth login
  --claudeai` can. A container profile says `connectors: shared` (Sandbox)
  or `none` (the default, Isolated). Each provider has one whole-account
  sign-in for containers, kept in `<config>/wsl-data/accounts/<provider>`
  and mounted as the container's whole `~/.claude`: the CLI saves its
  credentials by writing a new file and renaming it, so a link to a file
  doesn't keep them but a mounted folder does. Containers sharing
  connectors get it and no token (the token would win); others never get
  it, and it's unmounted if a profile stops sharing them. Anything running
  in such a container can use the sign-in, which Settings says before
  signing in; signing out deletes it.

## Coexisting with other WSL users (2026-10-08)

All WSL2 distros on a Windows account share one VM, one kernel and one
connection to the Windows drives, and other tools (Docker Desktop, Podman,
dev containers) live there too. A container's S:\ mounts were seen to die
("No such device") right after one of UNCLI's teardowns. UNCLI follows these
rules (Harvey's):

1. **Only its own distros.** It only ever terminates or unregisters `uncli-*`
   distros of its own container profiles, and **never runs `wsl --shutdown`**.
   The "Restart WSL" button is gone; a stuck WSL gets **Stop UNCLI's
   containers**, and Settings says that restarting Windows is the fix.
2. **By PID, never by name.** It stops only the `wsl.exe` processes it
   started, by their own PIDs.
3. **Gently.** It asks each process to end first: a CLI's input closes, and
   the process holding a container up is a shell waiting on its input. Only
   after a grace period (5 s) is it killed, by PID. A container's sessions
   stop before it's terminated, and it's terminated (`wsl --terminate`, of
   that distro) before it's unregistered. At exit UNCLI waits for its CLIs to
   end.
4. **Less churn.** Containers are long-lived. The lockdown (`/etc/wsl.conf`)
   goes into the image before import, so a build no longer stops and starts
   the distro to apply it.
5. **No global state.** It never edits `.wslconfig`, restarts WSL's services
   or mounts shared disks. The one global thing it does is put the user's
   default distro back after an import, because WSL makes the first imported
   distro the default.
6. **Mounts only inside its own distros**, at its own paths; `/mnt` is never
   touched.

`TestRealCoexist` checks it: another distro mounts a folder on S: and lists
it every half second while UNCLI builds, uses, rebuilds (with a session
running), stops and removes a container three times. No listing failed, and
the mount still worked afterwards. Docker and Podman weren't installed for
the test; the other distro's drvfs mount is the same mechanism theirs use.

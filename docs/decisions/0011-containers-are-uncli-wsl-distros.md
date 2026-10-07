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

# 0032. Containers keep paths in the project on their own disk

Status: accepted (alongside 0017 and 0028)
Date: 2026-11-12

## Context

A repository used from Windows and from an UNCLI container can't share
`node_modules`. `npm install` on Windows writes win32 native binaries
(esbuild, rolldown) and Windows junctions for workspace links, which the
container sees as broken links to `/mnt/s/...`. An install in the
container (Alpine, musl) writes Linux binaries Windows can't run. Each
side's install breaks the other's. It came up in the Cubewise Release
Manager repository, with its `release-manager` container.

UNCLI couldn't help. Every mount is a Windows folder (drvfs), a profile's
`folders:` mount only at `/home/uncli/<name>`, and 0017 says there are no
arbitrary Linux targets.

## Decision

- **Local paths.** A container profile, in the user's `containers.yaml` or
  a repository's suggestion (0028), may list `local:` paths: relative paths
  inside the session's folder, such as `node_modules` or
  `apps/web/node_modules`. Absolute paths, `..`, empty entries, `.git` and
  anything in it, names a Windows folder can't have, the same path twice
  and a path inside another listed one are refused, naming the entry.
  Paths compare without case, as Windows' folders do.
- **The container's own disk.** Each one is a folder on the distro's disk,
  `/var/lib/uncli/local/<project mount name>/<path>`, owned by the CLI's
  user, bind-mounted over the project mount at that path, as root, after
  the project is mounted. A path missing in the project is created there
  (an empty folder on Windows). A path that is a file, or that leads
  through a link, is refused rather than followed out of the project. The
  folders last between sessions and go with the container when it's
  deleted or rebuilt.
- **This is not a Linux target.** 0017's rule stands for Windows folders:
  a local path only puts the container's own disk under the project, at a
  path the project decides. Nothing of Windows is mounted anywhere new.
- **Mounted with the project.** Unmounting anything unmounts what's under
  it first, deepest first, so a project's local paths come off before the
  project does, whether it's replaced, changed or remounted after the
  distro restarted; mounting the project again binds them again. When
  UNCLI lets a container go (it quits, or the container is stopped,
  rebuilt or removed), the local paths are unmounted before the distro is.
  A path the profile no longer lists is unmounted at the next session
  start; its folder stays on the container's disk.
- **Listed and told.** The manifest (`/etc/uncli/mounts.json`) lists each
  one with the kind `local`, its path in the project as the name, and the
  folder on the container's disk as the source. The session's directive
  names them as the container's own, which Windows doesn't see.
- **Inside the container.** Local paths keep files in the container, not
  out of it, so they aren't among what 0028's import form calls reaching
  outside.

## Consequences

- A team's suggestion can say `local: [node_modules]` and both Windows and
  the container keep working installs of their own.
- What the container installs there is invisible from Windows: Windows
  tools see their own copy, or an empty folder.
- A session start with local paths costs one more `wsl.exe` call the first
  time they're bound, none after; letting a container go costs one when it
  has any.
- The folders on the container's disk aren't cleaned up when a project is
  no longer used; they go with the container.
- A session running in the container could race the bind by swapping a
  link into the path; the bind is checked afterwards and taken out again
  if it landed anywhere else.

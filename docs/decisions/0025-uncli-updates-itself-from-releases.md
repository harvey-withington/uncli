# 0025. UNCLI updates itself from its GitHub releases

Status: accepted
Date: 2026-10-28

## Context

New UNCLI releases had to be found and installed by hand. Since decision
0024, installing needs no administrator, and an install script hands the
installer to a scheduled task. That task quits UNCLI, installs and starts it
again, and is proven from a session on a PC reached from a phone. Harvey
(2026-10-28) wants UNCLI to update itself.

Releases are GitHub pre-releases, with `uncli-<version>-windows-x64-setup.exe`
and a `SHA256SUMS.txt` beside it (`scripts/release.mjs`). Release builds were
meant to carry their version, but `release.mjs` never stamped it, so every
build reported "dev". Test builds (publish-build, install-build) are newer
than the last release, not older.

## Decision

- **Which builds check.** Stamped builds. `release.mjs` stamps the whole
  version, pre-release part and all, into `internal/buildinfo` (`-ldflags
  -X`). A test build from `publish-build.mjs` carries the last release tag
  plus `+dev.<build>` (`0.3.0-alpha+dev.66`). Build metadata counts for
  nothing when versions are compared, so a test build checks as that
  release would: it's offered the next release, never the same or an older
  one, which lets the next release test updating on the PC that runs test
  builds. An unstamped build ("dev": `npm run build`, `wails dev`) never
  checks, and says it's updated with `npm run install-build`.
- **Finding one.** The releases API lists releases. The newest is the
  highest version (semantic versioning, pre-releases included, drafts not)
  that has both the installer and `SHA256SUMS.txt`. UNCLI checks 30 seconds
  after start and then every 6 hours, unless Settings → Updates says not to
  (`updateCheck`, on by default).
- **Asking, not installing by itself.** A newer release shows as a notice
  above the sidebar's footer, and atop the phone's session list: "UNCLI
  0.4.0-alpha is available", with Update now and What's new (its release
  page). Settings → Updates shows the same, with Check now. Harvey chose
  one click over installing unasked, which would restart UNCLI under a
  phone session mid-thought.
- **Installing.** Update now asks first, warning when an answer is in
  progress. It downloads the installer to `%LocalAppData%\UNCLI\update`,
  checks its SHA-256 against `SHA256SUMS.txt`, and deletes a file that
  doesn't match. Then it hands the installer to the install script
  (decision 0024), which UNCLI embeds, with a 3-second wait (a session
  running `npm run install-build` gets 20, to finish its reply). UNCLI is
  back in well under a minute. The phone retries every few seconds for the
  first half minute after a drop, and asks for the update status again when
  it reconnects, so it shows the new version as soon as the PC is back.
- **From the phone too.** A paired device sees the same status and can
  start the update (three RPC methods). Updating isn't setting up, and the
  PC is often reached only from the phone. When the phone reconnects to a
  server on another build than it loaded, it offers Reload, so its page
  matches the server.

## Consequences

- Users on a release get later releases with one click, at the desk or
  from the phone, and paired phones catch up with a tap.
- The installer's SHA-256 comes from the same release as the installer, so
  it catches a broken or partial download, not a compromised release.
  Signing the installer would close that, and is not done yet.
- A test build is installed with `npm run install-build`, as before, and is
  then offered the next release like any copy of the last one. A test build
  made from code newer than a release would still be offered that release,
  if a release were ever cut from an older commit.
- Only Windows can install this way. Elsewhere Update now says so.

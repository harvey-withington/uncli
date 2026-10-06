# 0007. Changed files come from snapshots, not a watcher

Status: accepted
Date: 2026-10-06
Refines: the brief's `watch/` package ("fsnotify: artifacts dir, touched files")

## Context

Each page lists the files Claude changed during its turn. Tools that write
(Write, Edit) say which file they wrote, but a command can change files too
(`npm run build`, `sed -i`, a script). The brief planned a file-system
watcher on each code session's repo, debounced and ignore-listed, to catch
those.

A watcher is a poor fit here:

- It sees every change, including the user's own edits in their editor and
  background tools (a dev server rebuilding), so it can't tell what Claude
  did during a turn from what happened around it.
- Recursive watching differs per OS. fsnotify has no recursive mode on
  Linux, so a large repo needs a watch per folder, and UNCLI may run many
  sessions at once.
- It needs its own ignore list, which would drift from the project's
  `.gitignore`.
- It would be a new dependency, and a goroutine and OS handles per session
  for as long as the session is open.

Two more findings on 2026-10-06:

- The CLI's Edit result carries `structuredPatch` hunks, so the line that
  changed can be found (fixture `edit-tool-result`).
- The parser reported a file when the tool was asked for, so a write the
  user denied still showed as touched.

## Decision

- **Tools:** the adapter reports a written file once its tool has
  succeeded, with the first changed line from an edit's patch. A denied or
  failed write isn't reported.
- **Commands:** at the start of a turn the session records each file in its
  folder (size and modification time). At the end of a turn that used a
  tool, it records them again and compares, and adds what changed to the
  page: changed or created by a command, or deleted. This runs in the
  background once the page has closed, and only the page's files columns
  are updated (`SetPageFiles`).
  - In a git repo the files are those `git ls-files --cached --others
    --exclude-standard` lists, so `.gitignore` decides.
  - Elsewhere a walk skips hidden and generated folders.
  - Past 20,000 files nothing is compared.
- One row per file on the page: a tool's word beats the snapshot's (it
  knows the line), creating beats editing, and a delete is final.

## Consequences

- Changes the user makes between turns aren't credited to Claude, and
  nothing runs while a session is idle.
- A file a command changes and then changes back within one turn isn't
  listed. Changes made after the turn ends, by a process Claude started,
  aren't listed either.
- The page's command-made changes show a moment after the answer finishes,
  not live while it streams.
- The artifact pane (decision record 0008) compares its folder the same
  way, at the same moments.
- No watcher package and no new dependency. `internal/snapshot` does the
  comparing; the brief's `watch/` isn't built.

# 0008. Artifact versions are kept by UNCLI, not in a git repo

Status: accepted
Date: 2026-10-06
Refines: the brief's artifact pane ("The artifacts folder is a git repo, auto-committed at turn end")

## Context

Chat and Co-work sessions put what they make (HTML pages, SVG, Mermaid
diagrams, documents) in `./artifacts/`. The artifact pane shows them, and
paging back should show each artifact as it was at that page. The brief
planned to make the artifacts folder a git repo, auto-committed at the end
of each turn, with each page linking to the commits it produced.

Git is the wrong tool here:

- UNCLI is for people who don't use a command line. Many have no git
  installed, and a Chat session's folder is UNCLI's own scratch folder.
- Every commit would spawn a git process, which on Windows needs care so a
  console window never shows, and a missing or old git would need handling.
- A `.git` folder would appear inside the user's Co-work folder, which may
  already be in a repo of its own (a nested repo, or a change to their
  `.gitignore`).
- Reading a version back means asking git again, for every artifact shown.

## Decision

- **Which sessions:** profiles with `artifacts: true` (Chat and Co-work by
  default), for the `artifacts` folder in the session folder.
- **When:** the session snapshots that folder (every file, nothing skipped,
  git or not) when a turn starts, and compares it at the end of a turn that
  used a tool, in the same background step as decision record 0007.
- **Versions:** each new or changed file is stored once, under its SHA-256,
  in UNCLI's data folder (`<config>/artifact-store/<2 hex>/<hash>`). The
  page records `{path, hash, size}` in `pages.artifacts`; a deleted file is
  recorded as deleted.
  - Files over 25 MB are recorded without a hash and shown only from the
    folder.
- **The pane** folds the pages up to the one shown into the artifacts as
  they were then. On the latest page, files no turn recorded (made before
  versions were kept, or by the user) are added from the folder as it is
  now.
- **Rendering:**
  - HTML runs in an iframe with `sandbox="allow-scripts"`. That gives it an
    opaque origin: no access to UNCLI's page, the Wails bridge or storage.
  - A Content-Security-Policy (`default-src 'none'`, inline script and
    style, `data:` and `blob:` media, `form-action 'none'`) blocks every
    network request.
  - Refresh and `base` tags are removed before display.
  - SVG and Mermaid (rendered by mermaid at its strict level) show the same
    way, with no scripts.
  - Checked in headless Edge on 2026-10-06: fetch, images from the network,
    the parent page, `window.parent.go` and storage are all blocked.

## Consequences

- No git needed, no processes spawned, nothing added to the user's folder.
  The same content is stored once, however many pages or sessions produce it.
- Versions live in UNCLI's data folder, not beside the files: the user can't
  browse them with git, and deleting a session leaves its versions behind
  until a clean-up is added (not done yet).
- Like changed files (0007), versions appear a moment after the answer
  finishes, and a file changed and changed back within a turn has no new
  version.
- A link clicked inside an HTML artifact can still navigate its own frame
  (the sandbox has no top navigation or popups). Whatever it loads stays in
  the sandbox.
- Mermaid adds about 700 kB (gzip 170 kB), loaded only the first time a
  diagram is shown. It brings KaTeX, which has a moderate prototype-pollution
  advisory at the time of writing.

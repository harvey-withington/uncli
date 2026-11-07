# 0028. A repository suggests containers; the user imports them

Status: accepted (supersedes 0023's layering, team rules, default container and trust prompt)
Date: 2026-11-06

## Context

Decision 0023 put a team's UNCLI setup in a repository's `.uncli/` folder:
container profiles, a default container for Code sessions, and team rules
(a lowest prompt level, no Unattended). Since a cloned repository isn't
trusted, it layered the repository's profiles between the built-ins and
the user's own, applied the rules without asking, and asked the user to
trust everything else in a prompt of its own, remembered per repository
against a checksum.

Designing that prompt showed what it costs: a trust store, a new kind of
question, read-only repository containers in Settings, ids namespaced per
repository, and a ban on a repository sharing the user's own things
(brain, MCP servers, connectors, secrets, folders), because a profile
trusted once kept running unseen. Yet a team container often should
suggest some of those, such as the user's brain or a secrets folder
(Harvey, 2026-11-06).

UNCLI already has a form that shows a container's whole setup and lets
the user change it: the container editor in Settings → Containers.

## Decision

- **Suggestions, not settings.** A repository's `.uncli/containers.yaml`
  (or `.yml`; with both, UNCLI uses neither and says so) suggests
  containers, in the format of the user's own `containers.yaml` with
  `version: 1`. Nothing in it applies or runs by itself.
- **Seen when a session gets a folder.** The backend looks from the
  folder up to the repository root (the folder holding `.git`). The new
  session dialog lists the suggestions there, with Import.
- **Importing is the trust step.** Import opens the container editor on a
  new container filled in from the suggestion. The user changes what they
  like, and saves (or saves and builds). The result is an ordinary
  container in their own `containers.yaml`, which remembers which
  repository and suggestion it came from, when, and a fingerprint of the
  suggestion as it was.
- **Calm unless it reaches outside.** Most suggestions are a team's
  ordinary setup and should be imported. So the form says so when nothing
  reaches outside the container, and is loud only about what does:
  sharing the user's own things, and setup steps UNCLI's command risk
  rules flag (sending data away, naming secrets, running fetched code,
  mounting drives), leaving out installing, admin rights and changing
  files, which a build does inside its own container anyway.
- **Syncing.** When the user's container differs from the suggestion, the
  dialog shows Sync, and says whether the project changed it (the
  suggestion's fingerprint differs from the one kept at import) or the
  user did. Sync is the import again, from scratch: the editor on the
  project's version, with the same warnings, replacing the user's
  container when they save it. UNCLI doesn't keep the user's own changes
  to carry them over.
- **Anything a container can do may be suggested,** including the user's
  brain, MCP servers, connectors, secrets and folders, since nothing
  applies until the user saves it.
- **Bases are UNCLI's built-ins.** A suggestion on any other base is shown
  as one UNCLI can't build, and can't be imported.
- **Containers stay as they are.** No layering of repository profiles, no
  repository containers in Settings, no team rules, no default container
  applied. An imported container that's built is picked for a session in
  that repository when nothing else was.
- **Desktop only.** The phone and an embedding host's UNCLI don't show
  suggestions. The phone runs sessions in containers already imported.
- **Kept from 0023:** the `.uncli/` folder at the repository root, its
  files carrying `version: 1`, no `.devcontainer` reading, nothing the CLI
  reads itself, and no secrets in the files.

## Consequences

- There's no trust store and no new kind of question: the container
  editor, which the user already knows, is where they see and decide.
- A teammate can ignore the team's container, or change it. That's what
  "suggestion" means, and it matches how `.devcontainer` is offered in VS
  Code ("Reopen in Container").
- A user who changes their copy sees Sync for as long as it differs, and
  makes their change again when they sync.
- Team rules are gone. Enforcing anything for a repository would need a
  decision of its own.
- The file format is a public contract other repositories depend on, as
  0023 said.

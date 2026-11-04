# 0027. Each host runs its own UNCLI, behind its own server

Status: accepted
Date: 2026-11-04

## Context

UNCLI's chat ships as `<uncli-chat>` (decision 0026), but nothing on the Go
side lets a host use it:

- The remote server serves only its own pages (decision 0022).
- Hosts have no credentials ("decided with embedding", 0022).
- Decision 0021 replaced the brief's phase 6 service (one UNCLI per user,
  on localhost, shared by every app) with each UNCLI serving its own
  sessions. It designed nothing for hosts.

A first design had host pages reach an installed UNCLI across origins. It
needed host approval, per-host tokens bound to origins, CORS and
local-network preflights, scoping sessions to each host, and a narrower
method list. All of that comes from one choice: the host's page talking to
a UNCLI that runs as a separate program, shared with other apps. The
brief's shared service would have had the same problems.

Harvey, 2026-11-04: the library was always meant to put a local UNCLI
inside another app's UI with minimal code, so developers don't reinvent an
AI chat harness and get UNCLI's features for free. Driving the standalone
app's sessions from another app isn't needed. There are no shared
sessions, and nothing global to the machine: a host's UNCLI is local to
that host and only that.

## Decision

- **Each host runs its own UNCLI core,** in its own process (Go) or as a
  sidecar program it starts (`uncli-core`).
  - The core listens only on `127.0.0.1`, on a random port, and accepts
    only a random key the host gives it at start.
- **The host's server forwards a path, such as `/uncli/`, to its core,**
  adding the key: a reverse proxy, or in Go `core.Handler()` mounted
  directly.
  - The page uses `<uncli-chat src="/uncli">`. It's same-origin, with no
    token in the page.
  - The browser never sees the core's port or key.
- **Who reaches that path is the host's business,** through its own
  sign-in. Approvals, prompt levels and the safe list apply as in UNCLI.
- **The core checks each request's `Origin`** against the host's origin,
  given at start. Another site can't drive it through the user's cookies
  for the host.
- **Nothing global to the machine.** All of a core's state lives in the
  host's data folder:
  - UNCLI's database, settings, scratch and downloaded CLIs;
  - each CLI's own config folder (sign-in, transcripts, memory, skills, MCP
    servers). A host may opt into the user's own CLI config.

  Containers, which are per Windows user, are left out of embedded cores
  until they can be named per host.
- **Remote access is the host's choice.** A host that wants a phone app
  switches it on for its own core. It's the same remote server as the
  standalone app's.
- **The Go side ships as `app/uncli`** (package `uncli`; not `embed`, which would clash with Go's own `embed` that hosts use for their assets), a public package in UNCLI's module:
  - `Start`, `Handler`, `Close`, and an optional `EnableRemote`;
  - `uncli-core` is a small program over it.
- **Helpers for other server stacks are optional.** Most set up a reverse
  proxy in a few lines. They ship only if hosts ask.
- **The standalone app's remote access is unchanged,** and serves only
  UNCLI's own phone app. Hosts never connect to an installed UNCLI.

This supersedes the brief's phase 6 "one local UNCLI service per user"
for hosts, and settles what decision 0022 left to embedding: hosts need no
credentials of UNCLI's beyond the key their own core is given.

## Consequences

- **No host approval, host tokens, CORS or session scoping.** The security
  questions reduce to the host's own sign-in and the core's origin check.
- **What has to be built:**
  - A real Go module path, replacing `uncli`.
  - `app/uncli`.
  - Links that resolve against the mount path, and Host and Origin checks
    that take the host's values.
  - `<uncli-chat src>`, and `<uncli-setup>` for installing and signing in
    to the CLIs.
  - `uncli-core`, and samples in Go and Node.
- **The user signs in to each host's CLIs separately,** and a host doesn't
  see their personal skills, MCP servers or memory unless it opts into the
  user's own config.
- **Each host carries a copy of the core** and its downloaded CLIs, and
  keeps it up to date with its own releases.
- **Sessions don't move between hosts,** or between a host and the
  standalone app.
- **Windows only** for now, like UNCLI.
- **What we gave up:** one place to see every session on the machine, and
  hosts that use an already installed UNCLI.

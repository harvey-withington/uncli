# 0021. Each UNCLI serves its own sessions over the tailnet

Status: accepted
Date: 2026-10-24

## Context

Harvey wants to run Code chats from an Android phone: see the same sessions
as the desktop app, send messages, answer approvals and start new sessions
with any CLI or settings (2026-10-24). Later (phase 2), the desktop app on a
laptop should show a home server's sessions beside its own, marked as
remote, and start sessions there. Later still, UNCLI's panel will be
embedded in other apps.

The brief's phase 6 assumed one local UNCLI service per user, listening on
localhost only. What constrains the design:

- A session's CLI runs on one machine and works in that machine's folders.
  Two home servers mean two UNCLIs, each with its own sessions.
- Tailscale already runs on every device on Harvey's home network.
- Android's Chrome installs a site as an app, and lets it receive push
  notifications, only over HTTPS with a valid certificate. Tailscale issues
  one only for a machine's tailnet name (`<machine>.<tailnet>.ts.net`), not
  for its `100.x` address. HTTPS certificates are already on in Harvey's
  tailnet.
- A Windows service runs as a system account, without the user's CLI
  sign-ins, settings, skills and MCP servers, and can't use UNCLI's WSL
  containers, which belong to a logged-in user.
- BRUV's server binds `0.0.0.0`, so the whole LAN can reach it, and relies
  on `tailscale serve` (set up by hand) for HTTPS.

## Decision

- **Every UNCLI can serve its own sessions.** It's the desktop app's own
  process, not a separate service. It runs as the user, in the tray: closing
  the window keeps it running, and it can start with Windows, minimised. A
  Windows service may come later and isn't part of v1.
- **A network transport over `app`,** beside the Wails bridge that serves
  the local window:
  - JSON-RPC over HTTP, dispatched by reflection to an explicit list of
    `app` methods, pinned by a test (BRUV's `transport/http/rpc.go`
    pattern);
  - a session event stream read with `fetch`, where each session's events
    carry a sequence number and a client that reconnects gets what it
    missed, or a full reload when they're gone.
- **Listening:** off until switched on in Settings → Remote access. When on,
  UNCLI listens on loopback and the machine's Tailscale address only, never
  the LAN or `0.0.0.0`.
- **HTTPS with the machine's own tailnet certificate.** UNCLI fetches it
  from the Tailscale app already running, renews it before it expires, and
  serves HTTPS on its own port, e.g. `https://<machine>.<tailnet>.ts.net:9871`.
  No embedded Tailscale node, and no `tailscale serve`. When there's no
  certificate (Tailscale not running, HTTPS certificates off in the
  tailnet), Settings says why.
- **The phone app is a PWA that UNCLI serves** at `/m/`:
  - the session list;
  - sessions with their pages, composer and approvals;
  - a new session with any provider, profile and model, in a folder the
    server's folder browser picks;
  - attachments uploaded to the server;
  - Web Push notifications.
- **Several clients on one session** see the same pages live. The first
  answer to an approval wins and closes it everywhere. A turn keeps running
  with no client connected, and a pending approval waits for one.
- **Phase 2 is designed in.** The desktop app will combine its own sessions
  with those of the servers it has paired with. So from v1:
  - The UI talks to UNCLI only through the `Backend` interface (Wails, HTTP,
    and in phase 2 one that combines several).
  - A session is identified by its server and its id.
  - The server's API covers everything the desktop app does with a session,
    so the phone and a remote desktop use the same calls.
  - A desktop pairs exactly as a phone does (decision 0022).
- **Remote access isn't part of the panel.** When UNCLI is embedded, the
  panel takes a `Backend` from its host and doesn't know whether it's local
  or remote. Serving is UNCLI's Go side; reaching a server is a client
  helper the host chooses. This decision doesn't design embedding further.

## Consequences

- One app per machine: install UNCLI, sign in its CLIs there, switch on
  Remote access, pair the phone. Nothing else to set up.
- The phone and, in phase 2, a laptop reach sessions only on machines that
  are on, logged in and running UNCLI.
- Sessions are split across machines. The phone opens one server at a time
  in v1; the desktop combines them from phase 2.
- UNCLI depends on the Tailscale app for its certificate. Whether a tray app
  running as a standard user can fetch it on Windows is the first thing to
  confirm when building.
- For a client that isn't on the server's machine, the following are
  disabled with the reason: open in editor, desktop-only features, and
  container and CLI management.
- This supersedes the brief's phase 6 "localhost only" and "one local
  UNCLI service per user".

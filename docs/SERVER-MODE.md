# Server mode (phase 6)

A draft that was written into `docs/BRIEF.md` (phase 6, "Headless API"
and the risks table) and taken back out on 2026-10-17, since the brief is
frozen. It builds on the brief's phase 6 headless API, panel controls and
security bullet. Nothing here is a requirement until it's on a BRUV card.

## Design

- **Security:** localhost only by default; a per-client token issued once
  the user approves the client in UNCLI; origin checks so web pages can't
  connect. CLI credentials never leave UNCLI, and every host gets the same
  approval rules. (The brief's bullet says "localhost only"; this draft
  adds "by default" for remote reach below.)
- **Remote reach:** an opt-in setting also listens on the Tailscale
  interface (never `0.0.0.0`), so UNCLI on a laptop away from home connects
  to UNCLI on the home PC with "Connect to…", instead of starting its own
  CLIs. Each device is approved once on the host, as a local client is, and
  its token can be revoked from the host's settings. The sessions, sign-in
  and CLI processes stay on the host; the laptop is only a view of them.
- **Reconnecting:** session events carry a per-session sequence number. A
  client that drops (sleep, Wi-Fi, travel) reconnects with the last number
  it saw and gets the events it missed, or a full reload of the session's
  state when the host no longer holds them. A turn keeps running on the
  host while no client is connected, and a pending approval waits for the
  next one.
- **Several clients on one session:** every client sees the same pages and
  approval cards. The first answer to an approval wins and the card closes
  everywhere; a message sent while a turn is running queues as it does in
  one window.
- **Host-local features:** anything that assumes the UI and the files share
  a machine is decided per feature. Files to attach are uploaded to the
  host rather than passed as a path. Folder pickers for new Co-work and
  Code sessions browse the host. Open in editor is hidden on a remote
  client (or opens a host path in an editor that understands remotes).
  Desktop notifications fire on the client that is connected, not on the
  host. Containers, provider installs and sign-ins are managed on the host
  and shown read-only remotely.
- **Version skew:** the host serves its version and build date. A client
  running different code says so in a banner, and the API refuses calls it
  doesn't know with an error the UI shows, rather than failing silently.
- **Not Claude's own remote sessions:** the CLI's remote control lets
  Anthropic's apps (desktop, claude.ai, mobile) pick up a local session
  through Anthropic's relay. That relay's client side has no public API,
  so UNCLI can't be one of its clients without reimplementing a private
  protocol and its sign-in, which breaks the official-binary rule. UNCLI
  reaches its sessions through its own server mode instead. Starting
  sessions with remote control enabled, so the Claude apps can join them,
  stays possible later if the headless mode supports it.

## Risks

| Risk | Impact | Mitigation |
|---|---|---|
| Server mode reachable beyond the PC | Anyone on the network drives the CLI as the user | Off by default; Tailscale interface only; per-device approval and revocable tokens; origin checks |
| Remote client and host on different versions | Calls fail or events are misread | Host serves its version; client shows a skew banner; unknown calls refused with a visible error |
| Connection drops mid-turn | Remote client misses output or an approval | Sequence-numbered events replayed on reconnect, full reload as fallback; turns and approvals wait on the host |

# 0020. MCP servers are signed in to from UNCLI

Status: accepted
Date: 2026-10-23

## Context

An MCP server that needs OAuth (BRUV, say) showed up only as missing
tools. Signing in meant doing it in another app, then restarting UNCLI's
sessions, and even that didn't always work (Harvey, 2026-10-23). Claude
Code 2.1.285 can do it headlessly, through control requests it doesn't
document. A spike against a local fake OAuth server recorded them
(fixtures `mcp-auth-*`):

- `mcp_status` gives each server's status (`needs-auth`, `connected`,
  `pending`, `failed`…) with its tools. No event says when it changes.
- `mcp_authenticate {serverName, redirectUri?}` answers with the link
  (`authUrl`) and how the browser comes back. The CLI never opens a
  browser. By default it listens on a localhost port, takes the code,
  stores the token in its config folder and reconnects by itself. With
  `redirectUri` the browser goes there instead, if the server accepts it.
  A connector of claude.ai's gives a link with no callback: you sign in
  on claude.ai, then reconnect.
- `mcp_oauth_callback_url {serverName, callbackUrl}` takes the address
  the browser came back to. The CLI then doesn't reconnect by itself, so
  the host sends `mcp_reconnect {serverName}`.
- `mcp_clear_auth {serverName}` signs out.
- Tokens live in the CLI's config folder. Another session's CLI using
  that folder connects on `mcp_reconnect`; one started later connects at
  once.

Other CLIs will have their own ways, or none.

## Decision

- **Provider-neutral seams.** `core.Capabilities.MCPSignIn`; controls
  `CtlMCPSignIn` (with an optional redirect), `CtlMCPCallback`,
  `CtlMCPReconnect` and `CtlMCPSignOut`, naming the server; events
  `EvMCPServers` (each server's status and tool count, in UNCLI's words)
  and `EvMCPAuth` (the link and how it returns, or an error, by step).
  The Claude adapter maps them to the requests above. It tags each
  request's id with its step and server, so an answer, an error included,
  goes to that server and never becomes the session's error. Any adapter,
  or an ACP or plugin driver, can implement the same controls and events;
  one that doesn't simply shows no MCP servers.
- **The session** keeps the servers in its view, with any sign-in
  waiting and the last failure. It asks for their status once the CLI has
  answered its handshake, at every turn, and every second while a
  sign-in waits for the browser (up to 10 minutes) or a reconnect settles
  (20 s). It starts the CLI to sign in if it isn't running. Nothing goes
  to the model.
- **Where the browser comes back to.** A local CLI takes it on its own
  port. A CLI elsewhere (WSL, a container) is asked to send it to UNCLI
  (`internal/oauthcatch`, on 127.0.0.1), which hands the address on and
  reconnects. If the server refuses that redirect, the CLI falls back to
  its own listener, which WSL's localhost forwarding usually reaches. The
  user can always paste the address their browser ended up on.
- **Sharing.** When a sign-in connects, the server is reconnected in the
  other running sessions on the same CLI in the same place that were
  waiting for it. One that keeps sign-ins of its own (sandboxed, another
  container) refuses quietly.
- **UI.** A line under the session's folder for each server that needs
  you: sign in; waiting for the browser (open the page again, paste the
  address, cancel); a connector to reconnect after signing in on the
  provider's site; couldn't connect (reconnect). It shows in either
  header mode. The header's plug button lists every server with Sign in,
  Reconnect and Sign out, and the palette has the same commands. A toast
  says when a server you acted on connects, with its tool count.

## Consequences

- Signing in to a server needs no other app and no restart. The other
  sessions get the server too.
- The requests aren't documented. A CLI update that changes them breaks
  this, and the fixtures and `TestMCPSignIn` (real CLI, fake server, no
  usage) say so before the pin moves.
- A sandboxed or container session signs in on its own; its sign-in isn't
  the user's everywhere.
- Status is polled, not pushed: a server that drops between turns shows
  as connected until the next check.
- Not yet tried against a real claude.ai connector or a real WSL session.

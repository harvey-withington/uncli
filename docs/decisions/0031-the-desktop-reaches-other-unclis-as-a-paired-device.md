# 0031. The desktop reaches other UNCLIs through its Go side, as a paired device

Status: accepted
Date: 2026-11-09

## Context

The desktop app shows a paired server's sessions beside its own (decision
0021, phase 2). The server only accepts requests from its own pages: it
checks each request's Origin, refuses every preflight and sends no CORS
headers (decision 0022). The desktop window is a page from another origin
(`http://wails.localhost`), so it can't call a server directly. Letting it
would mean CORS for that origin on every server and the device's token in
the window's storage.

## Decision

- **UNCLI's Go side is the paired device** (`remote.Peers`). It pairs with
  the server the same way a phone does, keeps the token in the operating
  system's credential store, makes the calls, and keeps each server's
  event stream open, passing the events up to the window. The window never
  sees a token, and never fetches another server itself.
- **The server doesn't change.** A request from Go carries no Origin, so
  the existing Host and token checks are all it meets. The remote API's
  method list stays the limit of what the desktop can do there.
- **The window combines backends** (`combinedBackend` in `@uncli/client`).
  Each server's calls run through the same method table as the phone's
  HTTP backend, over the bridge instead of `fetch`, and session ids are
  qualified by server (`withServer`), so the stores need no server
  awareness. The machine's own state (CLIs, providers, preferences,
  containers, usage) stays this PC's.
- **What needs the session's own machine** is decided per session, not per
  window (`atServer(backend, sessionId)`), and shown disabled with the
  reason.

## Consequences

- Tokens are never in the window's storage, and the server's
  cross-origin rules stay as strict as they are.
- Desktop notifications for remote sessions come from the same Go rules
  as local ones (`app.PeerNotes`).
- Everything a remote session does goes through one more hop (window to
  Go to server); for JSON calls on the tailnet that's negligible.
- Files attached on the desktop are read by Go and uploaded, so the
  window can keep handing over local paths.
- Removing a server forgets its token here; the server still lists this PC
  among its paired devices until it's revoked there.
- Credential Manager is Windows only, like UNCLI; another platform would
  need its own store before it can pair with a server.

## Alternatives

- **CORS for the desktop's origin.** Simpler in the window, but every
  server would trust any page claiming that origin, and tokens would live
  in the WebView's storage.
- **An embedded reverse proxy** on a loopback port for the window to
  fetch. Same effect as the bridge, with another listener to secure.

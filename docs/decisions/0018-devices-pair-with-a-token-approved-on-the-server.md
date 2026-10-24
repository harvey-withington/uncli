# 0018. Devices pair with a token approved on the server

Status: accepted
Date: 2026-10-24

## Context

With remote access on (decision 0017), any device on the tailnet can reach
UNCLI, and a paired device can make the user's CLIs run tools. Harvey's
tailnet is private, but it may hold shared devices, and a stolen token would
work from any of them.

BRUV's server is the model: a device pairs once and gets its own token. Its
weak points are the things to avoid:

- One shared bootstrap token enrols any device, with no approval on the
  server.
- Tokens travel in URLs: `?token=` on every route, because `EventSource`
  can't send headers, and the bootstrap token in the pairing page's address.
- CORS echoes any origin.
- The auth check is skipped for any path that contains `/attachments/`.

## Decision

- **Pairing is approved on the server.**
  - Settings → Remote access shows a QR code and a link. Both carry a
    pairing code that lasts a few minutes and works once.
  - The phone (or, in phase 2, a desktop) opens it and gives its name.
  - The server's window asks "Allow <name>?".
  - Once allowed, the device gets its own token. The server stores only its
    hash.
- **Settings lists paired devices,** with when each was last seen and a
  Revoke button. Revoking ends that device's open connections.
- **Tokens travel only in the `Authorization` header,** never in a URL.
  - The event stream is read with `fetch`.
  - Links to artifacts and attachments are signed, expire, and are checked
    on their exact route.
- **CORS allows only UNCLI's own origin.**
- **A paired device can do whatever the desktop window can on that server,**
  including answering approvals. Approvals, prompt levels and the safe list
  apply as they do locally: remote access never loosens them. Each approval
  answer records which device gave it.
- **Hosts are out of scope here.** Credentials for apps that embed UNCLI,
  and whether their panels may answer approvals, are decided with embedding.

## Consequences

- A phone pairs in a few seconds, and only with the user's say-so at the
  server.
- Pairing needs someone at the server's screen (or remote desktop). For a
  tray app that's acceptable. A Windows service would need another way to
  approve.
- No token ends up in browser history, logs or a Referer header.
- BRUV can adopt the same flow when it's hardened (BRUV card "Server
  hardening").

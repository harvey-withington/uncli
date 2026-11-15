# 0034. A paired device signs the PC's CLI in again

Status: accepted (widens 0021's methods for a paired device)
Date: 2026-11-15

## Context

The first provider's CLI signs in with a link and the code the page then
shows (Service.SignIn). Its sign-in expires now and then, and a turn that
fails on it shows Sign in again. Away from the PC that did nothing: on the
phone the call was refused before it left the device (setting up the CLIs
is off a paired device's methods, 0021), and on the desktop, for a paired
server's session (0031), it went to this PC's CLI rather than the server's.
The phone's Settings had no sign-in at all. Someone away for weeks loses
UNCLI when the sign-in runs out (Harvey, 2026-11-13).

The CLI's sign-in was kept at the PC as setup, like installing it, not as
a security boundary: a paired device can already run anything there through
a session (0033 makes the same point). What a remote sign-in adds is that it
could sign the CLI in to another account, which the PC's sessions would then
use.

## Decision

- **On the list:** `SignIn`, `SubmitLoginCode` and `CancelSignIn` are
  methods a paired device may call: the first provider's link-and-code
  sign-in, on the server. Installing a CLI, other providers' sign-ins and
  versions stay at the PC.
- **The PC is told:** a sign-in finished from a paired device shows a
  desktop notification on that PC naming the device and the account it's
  now signed in as, so a sign-in to another account doesn't go unseen.
- **The session's own PC:** Sign in again on a failed turn signs in the CLI
  of the PC the session runs on: this one, or a paired server's.
- **The link is offered, not opened:** the desktop's own sign-in still opens
  the link itself; anywhere else (the phone, a paired server's CLI) it's a
  button to open, since a browser blocks a page opened after waiting on the
  server.
- **The phone's Settings** shows the PC's CLI account with Sign in again.
- **Said once:** a turn that failed on the sign-in shows it in its card,
  with Sign in again; the session's header no longer repeats the error.

## Consequences

- A sign-in that expires while the user is away can be renewed from the
  phone or another PC.
- A lost phone or leaked pairing could sign the PC's CLI in to another
  account; the PC's notification is how the user would find out, and
  revoking the device (0022) stops it.
- The method list grows by three; `TestRPCMethods` pins it.

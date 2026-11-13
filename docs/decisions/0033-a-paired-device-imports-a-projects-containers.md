# 0033. A paired device imports and builds a project's containers

Status: accepted (widens 0021's methods for a paired device; amends 0028's
desktop-only rule for suggestions)
Date: 2026-11-13

## Context

A session can start on a paired UNCLI (0031), in a folder on that PC. When
that folder's repository suggests containers (0028), the new session
dialog didn't show them there, and Settings → Containers → Import from a
project could only pick this PC's folders: `SuggestedContainers`,
`ImportContainer` and `BuildContainer` were off the remote access server's
method list (0021), which a paired PC uses as a phone does. So a team's
container could only be imported sitting at that PC (Harvey, 2026-11-13).

The methods were left out because setup stays at a PC ("the phone is for
prompting"), not because they're a security boundary. A paired device can
already start a session in any folder on the server, answer its approvals
and turn on Unattended: in effect run anything there as the user. A
container imported remotely adds less than that: it can share the user's
secrets and folders with sessions in it, and its setup steps run as root
inside its WSL distro when it's built. What it does add is persistence and
quiet: a changed container keeps sharing in sessions the user starts
later, with nothing in the session list to show it.

Only allowing it from a paired PC, not a phone, was considered: it would
rest on what the device said when it paired, need PCs paired before then
to pair again, and a second method list, for no change to what a paired
device can already do.

## Decision

- **On the list:** `SuggestedContainers`, `ImportContainer` and
  `BuildContainer` are methods a paired device may call. Suggestions are
  read, and a container imported, only from folders on the server's own
  drives (not network paths), as for browsing and new sessions.
- **The PC is told:** an import, sync or build from a paired device shows
  a desktop notification on that PC naming the device, and the container
  remembers the device it was imported (or last synced) from. Settings →
  Containers says where each imported container came from, when, and from
  which device.
- **Where it's offered:** on the desktop, New session lists a project's
  containers on whichever PC the session starts on, and Import from a
  project asks where first (this PC or a paired server), then that PC's
  folder (its own folder dialog here, browsed on a paired server). A build
  started there is followed until it finishes, since a server's container
  changes aren't sent to this PC.
- **Not on the phone:** the phone's screens don't offer importing or
  building, though its device may call them; setup stays at a PC.
- **Not for an embedding host's page:** containers belong to the whole
  Windows user, not one host (0027), so a host's page isn't offered them.
- **Still only at the PC:** editing, removing, stopping and signing in
  containers, and turning WSL on.

## Consequences

- A team's container can be set up on another PC from the one in front of
  the user, as a session can be started there.
- A lost phone or a leaked pairing can change containers, as it could
  already run sessions; the notification and the "imported from" line are
  how the user would find out. Revoking the device (0022) stops it.
- The method list grows by three; `TestRPCMethods` pins it.

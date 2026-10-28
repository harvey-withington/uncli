# 0024. UNCLI installs for one user, in their own folder

Status: accepted
Date: 2026-10-28

## Context

UNCLI was installed for everyone, in `C:\Program Files`, so installing and
updating it needed an administrator: Windows asked for permission (UAC) on
the PC's own screen.

Since remote access (decision 0021), the PC running UNCLI is often reached
from a phone while nobody is at its desk. Harvey (2026-10-28) wants a new
build installed and UNCLI restarted from there, at the end of each feature,
by a session he's prompting. A UAC prompt can't be answered from the phone,
and a session can't install anyway: UNCLI runs its CLIs in a job that ends
with it (`internal/runtime/local`), and the installer quits UNCLI first.

Everything UNCLI keeps is already per-user: its data in `%AppData%\uncli`,
its CLIs in `%LocalAppData%\uncli`, the Start with Windows entry in the
user's own Run key. Nothing needs to be machine-wide.

Considered and refused: keeping Program Files and registering, once, a
scheduled task allowed to run the installer as an administrator. Any program
running as the user could then trigger an administrator install without a
prompt, which quietly gets around UAC.

## Decision

- **Where.** The installer installs for the user who runs it, in
  `%LocalAppData%\Programs\UNCLI`, the way VS Code's user installer and
  Chrome do. It asks for no administrator (`asInvoker`). Its uninstall
  entry, Start menu and desktop shortcuts are the user's own (`HKCU`, the
  user's folders).
- **Quitting the running copy.** The installer asks any running UNCLI to
  quit (`--quit`), wherever it's installed, not only one in its own folder,
  so the first per-user install also quits one in Program Files.
- **Moving from Program Files.** The old copy needs an administrator to
  remove. Run by hand, the installer offers to run the old uninstaller
  (Windows asks for permission). A silent install leaves it alone. That
  uninstaller works in the all-users folders, so the window state in the
  user's `%AppData%` is untouched. Start with Windows is rewritten at every
  start while it's on, so it names the copy that's running.
- **Installing from a session.** `scripts/install-build.ps1` copies an
  installer and itself to `%LocalAppData%\UNCLI\update` and starts a
  scheduled task of the user's own ("UNCLI update", no administrator). The
  task runs outside UNCLI's job. It installs silently, waits for every
  UNCLI to exit, starts the new one, and checks it stays up. If it doesn't,
  or the install failed, it starts whichever UNCLI is installed, so a PC
  reached from a phone is never left without UNCLI. It logs each step to
  `update.log` there. Remote access is a saved setting, so paired devices
  reconnect once UNCLI is back.

## Consequences

- Installing and updating never ask for permission, at the desk or from a
  phone. Another user on the same PC installs their own copy.
- A session that runs the update is stopped with UNCLI; its conversation
  resumes at the next message.
- A PC that had the Program Files copy has both until the old one is
  removed: two entries in Installed apps, and the old all-users shortcuts,
  which start the old version.
- Windows Firewall allows programs by path, and the new path has no rule of
  its own. Remote access listens only on Tailscale and loopback, and
  Tailscale's own rule lets its network in on Private networks, so nothing
  changes there.

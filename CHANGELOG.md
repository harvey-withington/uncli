# Changelog

What changed in each release of UNCLI, newest first. The release workflow
publishes a version's section as its release notes, so each heading is
`## <version>` exactly as tagged, without the v.

## 0.4.0-alpha

UNCLI updates itself, installs without an administrator, and the fixes
from the first days of using it from a phone.

### Updates

- **UNCLI updates itself.** It looks for a newer release when it starts
  and every few hours. When there is one, the sidebar (and the phone's
  session list) says "UNCLI <version> is available", with Update now and
  What's new. Update now downloads it, checks it against the release's
  SHA-256, then quits, installs and starts UNCLI again in under a minute.
  It asks first if an answer is in progress. Settings → Updates checks on
  demand, or turns the automatic checks off (decision 0025).
- **Start it from the phone, too.** Once the PC has updated, the phone
  offers Reload, so its app matches the PC without closing and reopening it.
- **Installs just for you.** UNCLI now installs in
  `%LocalAppData%\Programs\UNCLI` and never asks for an administrator. Run
  by hand over a copy in Program Files, the installer offers to remove it;
  Windows asks for permission once (decision 0024).
- Coming from 0.3.0-alpha, install this one by hand: 0.3.0-alpha can't tell
  it's out of date. Later releases come through Update now.

### Remote access

- A remote access button in the session header shows whether paired
  devices can reach this PC: green when they can, amber when only this PC
  can, red when it can't listen. It opens Settings → Remote access.
- On the phone, a session you're looking at no longer turns Unread when
  its answer finishes.
- Coming back to the phone app catches up even when the network isn't
  back yet, instead of showing "TypeError: Failed to fetch". "Couldn't
  reach the server" now fades by itself, and no message starts with an
  error's type name.

### Sessions

- MCP servers that need you are listed only in the plug button's drop-down,
  not as a line each under the header. The drop-down has Open the page
  again and Paste the address while a sign-in waits for the browser, and
  scrolls when there are many.
- When the agent switches branch (or pulls, merges or rebases), the files
  that changed with it fold into one line, "Switched from main to
  server-mode: 312 files differ", instead of being listed as the agent's
  own work.
- The side panel shows in a session with no answer yet, so its File tab is
  always there.
- Watch is greyed out, saying why, for a changed file outside the session
  folder.
- A file that can't be attached says why: it isn't there any more, or what
  Windows said when it couldn't be opened.

### Known issues

- The phone app still hasn't been tested on an iPhone.
- Attaching several files copied in Explorer failed once on one PC with
  "it can't be read". The message now says why, to find the cause.

## 0.3.0-alpha

Your sessions from your phone: UNCLI keeps running in the tray and, when
you switch Remote access on, serves them to the devices you pair, over
Tailscale only.

### Remote access

- **Settings → Remote access** serves this PC's sessions to paired
  devices. It's off until you switch it on, and then listens only on this
  PC and its Tailscale address, never your local network, with HTTPS from
  the machine's own tailnet certificate (decision 0021).
- **The phone app.** Open the address on a paired phone and add it to the
  home screen: the session list, each session's pages, approval cards and
  the message box, new sessions in any folder on the PC, and photos or
  files attached by sending them to the PC first. It reconnects by itself
  after the phone sleeps or the network drops, and catches up on what it
  missed. What only works at the PC (opening files and editors, setting up
  a CLI, containers) shows disabled, with why.
- **Pairing.** Pair a device shows a QR code with a code that lasts five
  minutes and works once. The device gives its name and waits; the PC asks
  "Allow <name>?" and says which machine on your tailnet is asking, and
  who's signed in there. Each device gets its own token, which never goes
  in a link, and Revoke shuts it out at once (decision 0022).
- **Notifications on the phone**, even with the app closed, when a
  session needs approval, finishes or fails. Each device picks all,
  approvals only or none, and tapping one opens its session.
- Approvals answered on a device say which device answered.

### The tray

- Closing the window keeps UNCLI running in the tray (Open UNCLI and Quit
  UNCLI on its menu). Settings → General can start it with Windows,
  minimised to the tray.
- `uncli.exe --quit` quits a running UNCLI as the tray's Quit does. The
  installer and uninstaller use it, so neither has to wait for you to
  close the app.

### Sessions

- **Restart** stops a session's CLI and starts it again on the same
  conversation: for a CLI that's stuck, or to pick up a container's
  changed folders. It's in the session header and the command palette,
  and in Options on the phone, and asks first if an answer is in progress.

### Known issues

- The phone app has been tested on Android. It hasn't been tested on an
  iPhone yet, where the keyboard may still push the session's header off
  the screen.

## 0.2.0-alpha

Signing in to MCP servers without leaving UNCLI, a live view of any file in
the session folder, slash commands, and answers to the agent's questions.

### Sessions

- **Watch a file.** The side panel's File tab (F) shows a file from the
  session's folder and updates as it changes: Markdown rendered, HTML and
  SVG as a page (sandboxed, with a button for the source), anything else as
  code. Find one by any part of its name, latest changed first
  ("todo" puts the newest TODO file on top); star the ones you come back
  to. Each page's changed files have a Watch button.
- **Slash commands.** Type / for a menu of the CLI's commands, Tab or Enter
  to complete; the command palette has them too. Commands that could switch
  prompting off are never offered or sent.
- **Questions and plans.** When the agent asks you something, answer on its
  card: its options, several picks, or your own words. Approve a plan, or
  keep planning with a note on what to change.
- **Efficient / Normal / Thorough** and **Prompt me** are one button each,
  which cycles on a click (Shift+click goes back), coloured by what it costs
  or risks. The status line and warnings sit in a strip under the header.
- Markdown in answers, artifacts and the File tab renders HTML (collapsible
  sections, key caps, images), cleaned of anything that could run, and
  task lists as checkboxes (decision 0019).
- Pages, watched files and artifacts remember where you were scrolled, also
  across restarts; a toggle under the scrollbar follows the end of an answer
  as it's written. What you type is kept per session through a restart,
  which reopens the session you had open.

### MCP servers and sign-in

- **Sign in to MCP servers from UNCLI.** A server that needs signing in says
  so; Sign in opens its page in your browser and its tools arrive without a
  restart, in your other sessions too. A plug button in the header lists
  every server, to sign in, reconnect or sign out; claude.ai connectors are
  signed in on claude.ai, then reconnected (decision 0020).
- **Chat can use your MCP servers, connectors and skills** while staying
  isolated from your CLI settings, chosen per chat (decisions 0015, 0016).
- Each session waits up to 15 s for its MCP servers to connect before the
  first message, so that message has all their tools.
- A turn whose sign-in failed or expired offers **Sign in again** and Send
  again, without restarting the app.

### Settings and containers

- **Conversation cache** (Settings → AI Providers): how long a session's
  conversation stays cached after its last message, with what each choice
  costs.
- Containers can share your secrets folder read-only at `~/.secrets` and
  mount folders of their own; read-only mounts really are read-only
  (decision 0017).

### Fixes

- Sending while offline fails at once with "not connected to the internet",
  instead of thinking for a long time and then blaming the sign-in.
- Clicking a notification, or launching UNCLI again, brings the window to the
  front without un-maximizing it.
- A background sub-agent's tool request is shown, not refused.
- UNCLI no longer keeps MCP servers' settings in its event log. The CLI's
  status replies include them, request headers and any tokens in them too.
  Those an earlier version kept are deleted when UNCLI starts; if one of your
  MCP servers uses a token in its headers, consider replacing it.
- Quitting UNCLI stops quick tasks in flight (safety judgements, page
  summaries) instead of leaving them running for a while after.

## 0.1.0-alpha

The first public alpha. UNCLI runs AI coding CLIs in a desktop app, so you can
work with them without a terminal. Windows 10 and 11 (x64).

### Sessions

- Chat, Co-work on a folder, or Code on a repository, several side by side.
- Each question and its answer is a page: page through them, bookmark them,
  pin the ones you want at hand, and copy any paragraph or code block.
- Switch model mid-conversation, toggle modifiers such as Efficiency Mode,
  and compact or inspect the context from the toolbar.
- Each page lists the files the agent changed; in Code sessions Open takes
  you to the line in your editor. Links to files in answers open them.
- Chat and Co-work keep artifacts (HTML, SVG, Mermaid, Markdown, images)
  version by version.
- Background work and sub-agents show on their session, and a turn the CLI
  starts on its own gets a page of its own.
- Import a conversation the CLI saved in the terminal and carry on in UNCLI.
- Archive finished sessions, see cost and usage over time, and find any
  action in the command palette (Ctrl+Shift+P). ? lists every shortcut.

### Approvals

- Prompt me (Always, When unsafe or Never) decides when the agent stops to
  ask before using a tool. Cards say what it wants to do; "This is safe"
  teaches UNCLI for next time.
- MCP tools are judged per server, and again when the server's version
  changes.
- No profile ever bypasses approvals. Where a CLI can't ask, UNCLI approves
  every tool call itself through a hook, and refuses the call if the hook
  fails.

### AI providers

- Claude Code and the Antigravity CLI built in. UNCLI downloads its own
  pinned, checksum-verified copy of each and never touches one you already
  have installed.
- Provider plugins add more CLIs with a `provider.yaml` and no code,
  including CLIs that speak ACP (the Agent Client Protocol). Grok Build is
  the first. A plugin does nothing until you enable it, after seeing what
  it would download and run.

### Containers

- Run a session in a locked-down Linux (WSL) that UNCLI builds and owns:
  only the session's folder is shared, with no Windows drives or programs.
- Built-in Sandbox (shares your memories, skills, MCP servers and, if you
  choose, connectors) and Isolated (shares nothing); edit them or add your
  own in Settings → Containers.

### Look and feel

- Light and dark themes, a `theme.yaml` to restyle UNCLI, and desktop
  notifications when a session you aren't looking at finishes or needs you.

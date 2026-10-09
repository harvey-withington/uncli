# Changelog

What changed in each release of UNCLI, newest first. The release workflow
publishes a version's section as its release notes, so each heading is
`## <version>` exactly as tagged, without the v.

## 1.0.0

The first release. UNCLI runs AI coding CLIs in a desktop app, so you can
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

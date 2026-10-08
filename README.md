# UNCLI

AI CLIs if you're not a CLI guy.

UNCLI is a desktop app for running Claude Code without a terminal. Run
several Claude sessions side by side (Chat, Co-work on a folder, or Code on
a repository), read each answer as a page you can bookmark and copy from,
and switch models or modifiers like Efficiency Mode with a click. It drives
the official Claude CLI with your Claude subscription; it never calls a
model API itself.

## Install

Download the latest release from
[GitHub Releases](https://github.com/harvey-withington/UNCLI-1.0/releases/latest).

On first run UNCLI downloads its own copy of Claude Code (the version this
release is tested with, about 240 MB, checked against Anthropic's published
checksum) and asks you to sign in with your Claude account in the browser.
It doesn't use or change any Claude Code you already have installed.

App data (sessions, pages, bookmarks) lives in your user config folder
(`%AppData%\uncli` on Windows); the downloaded CLI in your user cache folder
(`%LocalAppData%\uncli\cli`). Chat sessions get their own folder under
`%AppData%\uncli\scratch`.

To restyle UNCLI, put a `theme.yaml` in that folder: colours, fonts and
radii for light and dark, by the token names in the "Theme override" section
of `docs/UI-CONVENTIONS.md`. Run **Reload theme file** from the palette to
apply changes.

## Use

- **New session** (Ctrl+N): pick Chat, Co-work (a folder) or Code (a
  repository), and a model.
- Each question and its answer is a **page**. Page with ← and →, bookmark
  with B, and jump between bookmarks with the double arrows. Hover any
  paragraph, code block or the question to copy it (paragraphs copy their
  original markdown).
- The toolbar switches model mid-conversation, toggles modifiers (Use
  Agents, Efficiency Mode, Thorough), runs `/compact` and `/context`, and
  shows your subscription usage.
- **Prompt me** (Always, When unsafe or Never) decides when Claude stops to
  ask before using a tool. A card shows what it wants to do, and "This is
  safe" teaches UNCLI for next time. The shield opens the safe list.
- Each page lists the **files Claude changed**. In Code sessions, Open takes
  you to the line in your editor (Settings → Editor).
- Chat and Co-work sessions keep their **artifacts** (HTML, SVG, Mermaid,
  Markdown, images) version by version. The side panel's Artifacts tab (or
  A) shows them as they were at the page you're on, beside On this page (O).
- A **desktop notification** says when a session you aren't looking at
  finishes or needs approval (Settings → Notifications).
- **Import** a conversation the Claude CLI saved (New session → Continue a
  conversation from the terminal…) to read it in UNCLI and carry on. A
  deleted UNCLI chat can be brought back the same way.
- **Pin** pages you want at hand (P), **archive** finished sessions, see
  **usage** over time, and find any action in the **command palette**
  (Ctrl+Shift+P); ? lists every shortcut.
- **Containers** (Windows): run a session in a Linux that UNCLI builds with
  WSL, where Claude sees only the session's folder and can't start Windows
  programs. Settings → Containers turns WSL on (one administrator prompt and
  a restart), builds containers and signs them in. Then pick the container
  under Run in when you start a session. The built-in Sandbox shares your
  memories, skills, MCP servers and, once you sign in with your whole
  account (Settings → AI Providers), your claude.ai connectors; Isolated
  shares nothing. Containers are defined in `containers.yaml`: packages,
  `brain: shared` or `sandboxed`, `mcp` and `connectors: shared` or `none` (add your own
  in the config folder by `id`). Settings says when a container no longer
  matches its config and needs a Rebuild; conversations survive rebuilds.

## Building

Requirements: Node 24, Go 1.26 and the [Wails CLI](https://wails.io) v2.10.1.

```powershell
npm run dev               # run the app with hot reload
npm run build             # build it (app\build\bin\uncli.exe)
npm run check             # go vet and svelte-check
npm test                  # Go tests (parser over recorded CLI streams) and frontend tests
npm run test:integration  # real CLI: downloads the pinned version, uses your sign-in
npm run test:e2e          # real app and CLI on Windows, driven in headless Edge
npm run website           # landing page on http://localhost:5180
```

The npm scripts are the only entry points; VS Code tasks and CI call the
same ones. `test:integration` and `test:e2e` spend a little of your Claude
usage (Haiku turns) and are not part of `npm test`.

For development, `UNCLI_DATA_DIR` moves the app data elsewhere and
`UNCLI_CLAUDE_BIN` runs a specific Claude binary instead of the managed one.
`npm --prefix app/frontend run dev` runs the UI alone on a mock backend.

| Folder | What |
|---|---|
| `app/` | The app: Go backend (`internal/`), Svelte frontend (`frontend/`), built-in profiles (`config/defaults/`), recorded CLI streams (`testdata/streams/`) |
| `website/` | The landing page, published to GitHub Pages by `.github/workflows/deploy-pages.yml` |
| `docs/` | The build brief, UI conventions and decision records |
| `scripts/` | Build and dev helpers, and the end-to-end check |

## Licence

MIT

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

Phase 1 has no approval prompts: each mode has a fixed list of tools it may
use, and anything else is refused and shown in the page's tool list.

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

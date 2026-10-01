# UNCLI

AI CLIs if you're not a CLI guy.

<!-- Two or three sentences on what it does and who it's for. Then a screenshot:
![UNCLI](website/images/screenshot.png) -->

## Install

Download the latest release from
[GitHub Releases](https://github.com/harvey-withington/UNCLI-1.0/releases/latest).

<!-- Installer vs portable, first-run warnings (e.g. SmartScreen on unsigned builds), where settings live. -->

## Use

<!-- The two or three things people do first. -->

## Building

Requirements: Node 24 and whatever the app's stack needs (for Go + Wails:
Go 1.26 and the [Wails CLI](https://wails.io) v2.10).

```powershell
npm run dev        # run the app with hot reload
npm run build      # build it
npm run check      # type checks and linters
npm test           # tests
npm run website    # landing page on http://localhost:5180
```

The npm scripts are the only entry points; VS Code tasks and CI call the
same ones. See the `//` note in `package.json` for wiring them to a stack.

| Folder | What |
|---|---|
| `app/` | The main app |
| `website/` | The landing page, published to GitHub Pages by `.github/workflows/deploy-pages.yml` |
| `docs/` | UI conventions and decision records |
| `scripts/` | Build and dev helpers |

## Licence

MIT

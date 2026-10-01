# app

The main UNCLI app lives here. Delete this file before scaffolding
(most generators want an empty folder), then wire the root `package.json`
scripts (`dev`, `build`, `check`, `test`, `release`) to the new stack so
VS Code tasks, CI and Claude keep working unchanged.

## Go + Wails + Svelte (the usual)

```powershell
wails init -n UNCLI -t svelte-ts -d app
```

Then:

- Move the frontend to Svelte 5 and Vite's latest if the template lags, and
  add `svelte-check` and `vitest` (`check` and `test:run` scripts).
- Add an empty `app/frontend/dist/.gitkeep` so `go:embed` compiles on a
  clean checkout (the rest of `dist/` is ignored).
- Set `info` in `app/wails.json` (product name, company, version).
- Put pure logic in `internal/` (Go) and `src/lib/` (TypeScript), with tests.
- Start `app/frontend/src/locales/en.json`, the tokens in `app.css`,
  `ConfirmDialog` and toasts on day one; see `docs/UI-CONVENTIONS.md`.

Root `package.json` scripts:

```json
"dev": "cd app && wails dev",
"build": "cd app && wails build",
"check": "cd app && go vet ./... && npm --prefix frontend run check",
"test": "cd app && go test ./... && npm --prefix frontend run test:run"
```

## Other stacks

| Stack | Scaffold |
|---|---|
| SvelteKit web app | `npx sv create app` |
| Go CLI or service | `cd app; go mod init github.com/harvey-withington/UNCLI-1.0` |
| Browser extension, library | `npm init` in `app/`, bundled with Vite or esbuild |

Whatever the stack, keep the same shape: a `lib/` of tested pure logic, a
thin UI or CLI over it, and the root npm scripts as the only entry points.

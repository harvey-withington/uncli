# UNCLI

AI CLIs if you're not a CLI guy. Pronounced "un-clee". A desktop app
(Go, Wails, Svelte 5) that runs AI coding CLIs headlessly and gives them a
friendly UI: sessions, pages, approvals, artifacts.

## Layout

- `app/`: the Wails app. Go packages in `app/internal/` (core seams,
  adapters, runtimes, session, store, `app` service, `bridge`), the Svelte
  frontend in `app/frontend/`, built-in YAML in `app/config/defaults/`,
  recorded CLI streams in `app/testdata/streams/`.
- `docs/`: `BRIEF.md` (the frozen design handover), `decisions/`
  (architecture decision records, extending the brief),
  `UI-CONVENTIONS.md` (the frontend contract), `PLUGINS.md` (the provider
  plugin reference).
- `website/`: the landing page. `scripts/`: build, release and test
  helpers.
- `plan/`: a private scratchpad, its own git repo, ignored here.

`npm run dev | build | check | test` are the entry points for every stack.

## Before working here

- Changing code, docs or plans, or writing a commit message: read the
  `conventions` skill (`.claude/skills/conventions/SKILL.md`).
- Seeing a UI change rendered: use the `run-app` skill.
- Requirements live on the UNCLI project on BRUV; its id is in Claude's
  memory for this project.

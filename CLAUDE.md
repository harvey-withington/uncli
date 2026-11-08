# UNCLI

AI CLIs if you're not a CLI guy. Pronounced "un-clee". A desktop app
(Go, Wails, Svelte 5) that runs AI coding CLIs headlessly and gives them a
friendly UI: sessions, pages, approvals, artifacts.

## Layout

- `app/`: the Wails app. Go packages in `app/internal/` (core seams,
  adapters, runtimes, session, store, `app` service, `bridge`), the Svelte
  frontend in `app/frontend/` (the app in `src/`; the reusable packages
  in `packages/`, an npm workspace: `@uncli/client` and `@uncli/chat`,
  the chat's components and elements; decision 0026), built-in YAML in `app/config/defaults/`,
  recorded CLI streams in `app/testdata/streams/`.
- `docs/`: `BRIEF.md` (the frozen design handover), `decisions/`
  (architecture decision records, extending the brief),
  `UI-CONVENTIONS.md` (the frontend contract), `PLUGINS.md` (the provider
  plugin reference), `EMBEDDING.md` (`<uncli-chat>` for host authors).
- `website/`: the landing page. `scripts/`: build, release and test
  helpers.
- `plan/`: a private scratchpad, its own private repo
  (`harvey-withington/uncli-plan`, cloned here as `plan/`), ignored here.

`npm run dev | build | check | test` are the entry points for every stack.

## Before working here

- Changing code, docs or plans, or writing a commit message: read the
  `conventions` skill (`.claude/skills/conventions/SKILL.md`).
- Seeing a UI change rendered: use the `run-app` skill.
- Requirements live on the UNCLI project on BRUV. Its ids, how Harvey
  works and the build routine are in `plan/CLAUDE.md` (private), imported
  below; nothing about working here relies on one machine's Claude memory.

@plan/CLAUDE.md

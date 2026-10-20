---
name: conventions
description: UNCLI's contributor conventions - architecture rules, frontend contract, tests and checks, where requirements, plans and docs go, and commit message format. Use before changing code, docs or plans in this repo, or writing a commit message.
---

# UNCLI conventions

Read this before changing code. The README says what the app is; this says
how changes are made so they fit. The general rules (strict types, no `any`,
localized strings, design tokens, no native `confirm()`/`alert()`, ID-keyed
state, ~300-line components) come from the global coding conventions and are
not repeated here; this file holds what is specific to UNCLI.

## Architecture

`docs/BRIEF.md` describes the design; these are the rules a change must not
break (`internal/core/seams_test.go` enforces the first two):

- `internal/core` imports nothing from UNCLI. New providers are adapters,
  new places to run are runtimes; neither changes `core` beyond adding event
  kinds or capability flags.
- Only `internal/bridge` imports Wails. Everything the UI calls lives in
  `internal/app`, which a different bridge (an HTTP server, say) can reuse.
- The UI never parses provider JSON: it sees UNCLI's events and pages
  through `frontend/src/lib/api`. Provider details stay in the adapter.
- Fixtures are the contract. A new CLI behaviour gets a recorded stream in
  `app/testdata/streams/<provider>/<version>/` and a parser test before
  anything depends on it. Moving to a new CLI version means recording
  fixtures, passing the tests, then bumping `PinnedVersion`.
- UNCLI runs only its own pinned, checksum-verified CLI (or one the user
  chose at their own risk). Never bypass permissions in a profile default.
  An adapter may switch off its CLI's own checks only when UNCLI gates every
  tool call itself and fails closed: a hook that crashes, times out or
  answers nonsense blocks the call, with a test for each (decision 0012).
- Config over code: profiles, modifiers and the toolbar are YAML in
  `app/config/defaults/`, overridable by `id` from the user's config folder.
- Pure logic lives in Go packages and `frontend/src/lib/`, with unit tests;
  components get plain values from the store.

## Frontend

`docs/UI-CONVENTIONS.md` is the contract: tokens, components, props and
keyboard behaviour. Update it in the same change as any shared component or
pattern.

## Tests and checks

Run before calling a change done:

```
npm run check
npm test
```

- A UI change is not done until it has been seen rendered: use the `run-app`
  skill and look at the screenshot.
- A change to the adapter, session or bridge also runs
  `npm run test:integration` (real CLI); a change that affects the
  definition of done runs `npm run test:e2e`. Both use the user's Claude
  usage, so say when you run them.
- Pure logic gets a unit test. Components get a smoke test that renders real
  data and asserts the text a user relies on.

## Commit messages

One line: a conventional-commit prefix (`feat`, `fix`, `refactor`, `docs`,
`test`, `chore`, `ci`), then the changes as brief clauses separated by
` / `, no body.

```
feat: drag to reorder / empty state for the library
fix: settings no longer lost on a failed save
```

## Text meant to be copied

Anything the user is meant to copy and paste (a suggested commit message, a
command, a card comment, a snippet) goes in its own fenced code block, so
the chat shows a copy icon on it. Write it at full line width: no hard line
breaks inside a sentence or paragraph, only between paragraphs or list items.
Claude doesn't commit or push in this repo; it suggests the message this way
and the user commits. The suggestion covers everything uncommitted since the
last commit (check `git log -1` and `git status`), not only the latest
change: the user commits once a batch of work is done.

## Requirements, planning and working rules

- Requirements and next steps live on the UNCLI project on BRUV (its id is
  in Claude's memory). A feature is fleshed out on its card as it's built.
- `plan/` is a private scratchpad: its own git repo, ignored here. Designs,
  drafts, proposed decisions, notes and TODOs go there as dated files
  (`YYYY-MM-DD topic.md`), and a design is also attached to its card.
  Nothing in `plan/` is a requirement until it's on a card. Planning never
  goes in this repo's `docs/`.
- Read `docs/BRIEF.md` before structural work, and follow its "Working rules
  for the builder" alongside this file. If the two ever disagree, stop and
  flag it rather than picking one. The brief is frozen (final as of
  2026-10-04): don't edit it for new work.
- Work one task at a time, and tick off the phase's definition of done as
  items pass.
- If the CLI behaves differently from what the brief assumes, record it on
  the relevant card (and in a decision record if it changes the design)
  before changing the code.

## Documentation

What's public in this repo, and nothing else:

- `README.md`, for people who run or contribute to the app.
- `docs/BRIEF.md`, the frozen design handover.
- `docs/decisions/`, decision records, which extend the brief. Write one
  when a choice shapes the architecture or rules out an alternative for a
  reason a later reader would otherwise have to rediscover. Only accepted
  decisions go here; a proposal stays in `plan/` and on its card until
  Harvey rules on it.
- `docs/UI-CONVENTIONS.md`, the frontend contract.
- Reference for people extending UNCLI, such as `docs/PLUGINS.md` for
  plugin authors.

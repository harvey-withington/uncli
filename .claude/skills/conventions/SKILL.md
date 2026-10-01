---
name: conventions
description: UNCLI's contributor conventions - architecture rules, frontend contract, tests and checks, and commit message format. Use before changing code in this repo or writing a commit message.
---

# UNCLI conventions

Read this before changing code. The README says what the app is; this says
how changes are made so they fit. The general rules (strict types, no `any`,
localized strings, design tokens, no native `confirm()`/`alert()`, ID-keyed
state, ~300-line components) come from the global coding conventions and are
not repeated here; this file holds what is specific to UNCLI.

## Architecture

<!-- The rules a change must not break. Replace these examples:
- `core/` never imports `platform/`; a new OS is a new provider.
- Pure logic lives in `lib/` and is unit-tested; components get plain values.
-->

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

## Documentation

- `README.md` is for people who run or contribute to the app.
- `docs/decisions/` holds decision records; write one when a choice shapes
  the architecture or rules out an alternative for a reason a later reader
  would otherwise have to rediscover.
- Requirements and next steps live on the BRUV card. Planning notes and
  TODOs live in `plan/`, never in this repo.

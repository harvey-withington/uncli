# UNCLI UI conventions

This is a contract. Update it in the same change as any shared component,
token or pattern, and keep the prop tables and keyboard behaviour in step
with the code. Delete it if the project has no UI.

## Ground rules

- Svelte 5 runes, typed props via `interface Props`, strict TypeScript,
  never `any`.
- Every user-visible string goes through `t('key')`; keys live in
  `locales/en.json`.
- Colours and sizes are CSS custom properties in `app.css`, defined for
  light and dark. Components never hardcode a colour, and colour is never
  the only signal.
- No component over about 300 lines; extract a child when one grows.
- Errors the user should know about are shown (toast or inline), never only
  logged. No `alert()` or `confirm()`: destructive actions go through
  `ConfirmDialog`.
- Keyed `{#each}` blocks use stable ids, never the index.
- Shared DOM behaviours (click-outside, focus trap, drag) are actions in
  `lib/actions.ts`. Reordering is drag and drop, not up/down buttons.

## Tokens

| Token | Use |
|---|---|
| `--bg` | Window background |
| `--surface` | Panels and cards |
| `--text` / `--text-muted` | Body and secondary text |
| `--border` | Dividers and outlines |
| `--accent` | Primary actions, focus, selection |
| `--danger` | Destructive actions and errors |

## Components

| Component | Props | Notes |
|---|---|---|
| `ConfirmDialog` | `title`, `message`, `confirmLabel`, `danger?` | Resolves `true`/`false`; Enter confirms, Escape cancels |
| `Toasts` | none; driven by `showToast(message, kind)` | Errors stay until dismissed |

## Keyboard

| Keys | Where | Does |
|---|---|---|
| Escape | Dialogs, popovers | Closes the topmost layer only |

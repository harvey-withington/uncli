# UNCLI UI conventions

This is a contract. Update it in the same change as any shared component,
token or pattern, and keep the prop tables and keyboard behaviour in step
with the code.

## Ground rules

- Svelte 5 runes, typed props via `interface Props`, strict TypeScript,
  never `any`.
- Every user-visible string goes through `t('key')`; keys live in
  `locales/en.json`. A test fails if a static key is missing.
- Colours and sizes are CSS custom properties in `app.css`, defined for
  light and dark. Components never hardcode a colour, and colour is never
  the only signal (activity states always have a text label).
- No component over about 300 lines; extract a child when one grows.
- Errors the user should know about are shown (toast or inline), never only
  logged. No `alert()` or `confirm()`: destructive actions go through
  `ConfirmDialog` via `confirm()` in `lib/confirm.svelte.ts`.
- Keyed `{#each}` blocks use stable ids, never the index (toolbar items,
  which have no ids and never reorder, are the one exception).
- Shared DOM behaviours (click-outside, focus trap, autosize) are actions in
  `lib/actions.ts`.
- The UI never parses provider JSON. It sees only UNCLI's events and pages
  through `lib/api` (the `Backend` interface), implemented by the Wails
  bridge in the app and by a mock in a plain browser.
- Model output is untrusted: markdown renders with raw HTML off, and links
  open in the user's browser through the backend, never in the app window.
- Motion is subtle (120–200 ms, `--ease`) and disappears under
  `prefers-reduced-motion`.

## Tokens

| Token | Use |
|---|---|
| `--bg` | Window background, sidebar, page area |
| `--surface` | Panels, cards, dialogs, header |
| `--surface-2` / `--surface-3` | Hover fills, code blocks / inline code, meters |
| `--text` / `--text-muted` / `--text-faint` | Body, secondary, hints and placeholders |
| `--border` / `--border-strong` | Dividers and outlines / emphasised outlines |
| `--accent` / `--accent-hover` / `--accent-soft` / `--accent-text` | The one accent: primary actions, focus, selection, active toggles / text on accent |
| `--danger` / `--danger-soft` | Errors, denials, destructive actions |
| `--warning` / `--warning-soft` | Interrupted turns, at-your-own-risk notes |
| `--success` | Copied, tool succeeded |
| `--shadow-sm` / `--shadow-md` / `--shadow-lg` | Cards / popovers and toasts / dialogs |
| `--scrim` | Behind dialogs |
| `--font` / `--mono` | Inter Variable / JetBrains Mono Variable (bundled; no ligatures in code) |
| `--text-xs` … `--text-xl` | Type scale (11.5, 12.5, 14, 15.5, 20 px) |
| `--space-1` … `--space-7` | Spacing (4, 8, 12, 16, 24, 32, 48 px) |
| `--radius-sm` / `--radius` / `--radius-lg` | 6, 10, 14 px |
| `--sidebar-width` / `--reading-width` | 272 px / 760 px |
| `--ease`, `--fast`, `--normal` | Motion curve and durations |

Shared control classes in `app.css`: `.btn` with `primary`, `ghost`,
`danger`, `icon`, `small`; `.input`; `.select`; `.visually-hidden`.

## Components

| Component | Props | Notes |
|---|---|---|
| `Modal` | `title`, `width?`, `onclose`, `children`, `footer?` | Focus trap; `data-autofocus` picks the first focus; Escape closes; click on the scrim closes |
| `ConfirmDialog` | none; driven by `confirm({title, message, confirmLabel, danger?})` | Resolves `true`/`false`; Enter confirms, Escape cancels |
| `Toasts` | none; driven by `showToast(message, kind)` | Errors stay until dismissed |
| `Icon` | `name`, `size?`, `label?`, `spin?` | Lucide names as used in YAML; unknown names fall back to a neutral glyph |
| `CopyButton` | `text`, `label?` | Copies through the backend; shows a check for 1.4 s |
| `ActivityBadge` | `state`, `showIdle?` | Dot plus label; pulses while busy |
| `Sidebar` / `SessionItem` | — / `session`, `active` | Double-click or pencil renames; trash confirms then deletes |
| `NewSessionDialog` | none | Profile cards, folder picker for co-work and code, model select |
| `SessionPane` | `session` | Header, toolbar, current page, nav bar, composer |
| `Toolbar` | `session` | Rendered from `toolbar.yaml`: model picker, modifier toggles (groups exclusive), slash and native items |
| `UsagePopover` | `usage`, `onclose` | Subscription windows from `EvUsageLimit` |
| `PageView` | `page` | Sticky question with copy, streamed answer, banners, trace, chips |
| `AnswerBlocks` | `markdown`, `streaming?` | One block per top-level markdown element; each copies its source markdown |
| `CodeBlock` | `code`, `lang`, `streaming?` | Shiki highlighting once complete; copies code only |
| `TraceStrip` | `items` | Collapsed summary with running, denied and failed counts |
| `PageChips` | `page` | Model, modifiers, tokens, cost, duration |
| `NavBar` | none | Previous bookmark, back, position, forward, next bookmark, bookmark toggle |
| `Composer` | `session` | Per-session drafts; Stop while answering |
| `SetupScreen` | none | Download the pinned CLI, then sign in |
| `SettingsDialog` | none | CLI version at the user's own risk |

## Keyboard

| Keys | Where | Does |
|---|---|---|
| Escape | Dialogs, popovers | Closes the topmost layer only |
| Enter / Shift+Enter | Composer | Send / new line |
| ← / → | Session (not while typing) | Previous / next page |
| B | Session (not while typing) | Toggle bookmark on the current page |
| Ctrl+N (⌘N) | Anywhere once set up | New session |
| Enter | Rename field | Save; Escape cancels |

## Running the UI without the app

`npm --prefix app/frontend run dev` (or `vite preview` after a build) runs
the UI on the mock backend. `?mock=setup`, `?mock=signin` and `?mock=empty`
pick a starting scenario; `?theme=light` or `?theme=dark` forces a theme
for screenshots.

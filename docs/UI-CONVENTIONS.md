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
- Shared DOM behaviours (click-outside, focus trap, autosize, drag) are
  actions in `lib/actions.ts`.
- **Drag and drop wherever it makes sense.** Lists the user orders are
  reordered by dragging (`dragSort`: a few pixels of movement starts it; a
  lifted ghost follows the pointer while the others slide aside to open the
  slot it will land in, which is the drop indicator; on release the ghost
  settles into the slot; Escape cancels; the scroller follows near its edges;
  the click after a drag is swallowed; reduced motion skips the animation),
  never by up/down buttons, and always have a keyboard twin
  (Alt+↑ / Alt+↓ on the focused item). Panel edges resize by dragging
  (`ResizeHandle`). Files and folders dropped from the OS do the obvious thing
  where they land (`lib/drops.ts`): onto the composer they attach to the next
  turn as chips (files copied in Explorer and pasted there too, and pasted
  screenshots; folders and files that can't be attached go in as their path,
  with a note saying why), a folder anywhere
  else starts a new session there. Controls inside a draggable item carry
  `data-no-drag`.
- **Colour comes in splashes, fully saturated.** Icons and accents that
  distinguish things (section kinds, session types) use HSL at 100%
  saturation and medium-to-dark lightness (lighter in dark themes; nudge it
  per hue so a set reads evenly), never greyed or pastel. Session types take
  a `hue` from their profile YAML and render through `tintStyle()`
  (`--tint`, `--tint-soft`, with the theme's `--tint-l`); anything without its
  own colour falls back to the one app accent. Colour is still never the
  only signal.
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
| `--tint-l` | Lightness for type colours (40% light, 62% dark); `tintStyle(hue)` sets `--tint` and `--tint-soft` on an element |
| `--kind-<kind>` | Outline section-kind icons, one per kind in `lib/sections.ts`: full HSL saturation, mid-dark (lighter in dark themes) |
| `--font` / `--mono` | Inter Variable / JetBrains Mono Variable (bundled; no ligatures in code) |
| `--text-xs` … `--text-xl` | Type scale (11.5, 12.5, 14, 15.5, 20 px) |
| `--space-1` … `--space-7` | Spacing (4, 8, 12, 16, 24, 32, 48 px) |
| `--radius-sm` / `--radius` / `--radius-lg` | 6, 10, 14 px |
| `--reading-width` | 760 px (the sidebar and outline widths are user-resizable, see `ResizeHandle`) |
| `--ease`, `--fast`, `--normal` | Motion curve and durations |

Shared control classes in `app.css`: `.btn` with `primary`, `ghost`,
`danger`, `icon`, `small`; `.input`; `.select`; `.visually-hidden`.

## Components

| Component | Props | Notes |
|---|---|---|
| `Modal` | `title`, `width?`, `onclose`, `children`, `footer?` | Focus trap; `data-autofocus` picks the first focus; Escape closes; click on the scrim closes |
| `ConfirmDialog` | none; driven by `confirm({title, message, confirmLabel, danger?})` | Resolves `true`/`false`; Enter confirms, Escape cancels |
| `Toasts` | none; driven by `showToast(message, kind)` | Top right, never over the message box; errors stay until dismissed |
| `CopyButton` | `text`, `label?` | Copies through the backend; shows a check for 1.4 s |
| `ActivityBadge` | `state`, `showIdle?` | Dot plus label; pulses while busy |
| `ResizeHandle` | `edge`, `width`, `min`, `max`, `label`, `onresize`, `oncommit` | A separator on a panel edge: drag, or arrow keys when focused (Shift for bigger steps). Used by the sidebar (200–440 px) and outline (180–480 px); widths kept in localStorage |
| `ApprovalCard` | `session`, `approval` | A tool use waiting for the user, above the page controls (`role=alertdialog`): what Claude wants to do in plain words (`lib/approvals.ts`: command, file and content, edit as a diff, MCP tool and server), then Allow, Always allow with a choice of rule (most specific first) and of scope (for this session, the default, or in this project), and Deny. Never takes focus: the user may be typing |
| `PermissionsDialog` | none (current session) | This session's rules first (remove, Make permanent), then the project's tool rules (the session's folder): each with Allow / Ask / Deny and remove; add a rule for a git class, a command prefix or a tool. Opened from the shield in the session header |
| `SearchPanel` | `results?` | The search box under New session (Ctrl+K) with filters for type, bookmarked, this session and date; while it has text, the sidebar shows grouped results (`results` mode) instead of sessions. Snippets come with \x01/\x02 markers, never HTML. Opening a result selects the page and scrolls to the first matching block, which flashes (`.search-flash`) |
| `Sidebar` / `SessionItem` | — / `session`, `active` | Drag to reorder (or Alt+↑ / Alt+↓); type icon and active stripe in the type's colour; double-click or pencil renames; trash confirms then deletes |
| `NewSessionDialog` | none | Profile cards, folder picker for co-work and code, model select. Opens on the welcome card clicked, else the last type used; each type starts from its last model and folder (remembered in the app database) |
| `SessionPane` | `session` | Header, toolbar, current page, nav bar, composer. The header's Unattended toggle (coffee icon, `aria-pressed`, warning tint when on) declines whatever would ask the user; while on, a status line under the folder says so |
| `Toolbar` | `session` | Rendered from `toolbar.yaml`: model picker, modifier toggles (groups exclusive), slash and native items |
| `UsagePopover` | `usage`, `onclose` | Subscription windows from `EvUsageLimit` |
| `PageView` | `page` | Sticky question with copy, the files sent with it (`q-files`), streamed answer, banners, trace, chips. A chevron collapses the question to compact mode: one line, the question left (ellipsis when short of room) and the page number right, never cut, then copy and the chevron; kept in localStorage |
| `AnswerBlocks` | `markdown`, `streaming?`, `markers?` | One block per top-level markdown element; each copies its source markdown. `markers` (block index → outline entry, from `app.outlineFor(page)`) puts the entry's section-kind icon in the page's left margin beside the block, 16 px in the kind colour (a step up from the outline's 13 px), so the margin always matches what the outline shows, Summary/Headings switch included |
| `CodeBlock` | `code`, `lang`, `streaming?` | Shiki highlighting once complete; copies code only |
| `OutlinePanel` | `page`, `scroller` | "On this page": the answer's headings (levels normalised; paragraphs that act as headings, such as "**1. Point.** …" or a short all-bold line, count one level below the real heading before them), or a summary from the quick-task model (sparkles button; Summary/Headings switch when both exist; caption names the model and cost; summaries also start on their own as answers finish, per the Summaries setting, and an answer skipped as too short says so with a link to summarise it anyway), each entry with a section-kind icon (`lib/sections.ts`: guessed for headings, chosen by the model in a summary; the kind is in the tooltip and read out to screen readers), the question at the top; click to jump below the sticky question, the section being read is highlighted. Resized by dragging its left edge or arrow keys on it (180–480 px); width and visibility kept in localStorage |
| `TraceStrip` | `items` | Collapsed summary with running, denied and failed counts |
| `PageChips` | `page` | Model, modifiers, tokens, cost, duration |
| `NavBar` | none | Separate rounded buttons (gradient fill, filled icons, no text): previous bookmark ◀◀, back ◀, position capsule "3 / 7" (read as "Page 3 of 7"), forward ▶, next bookmark ▶▶, bookmark toggle (filled when set). Docked by `SessionPane` on the line between answer and composer, centred over the answer column; the answer fades out above it |
| `Icon` | `name`, `size?`, `label?`, `spin?`, `fill?`, `flip?` | `fill` for solid shapes, `flip` to mirror (a left triangle is a flipped play) |
| `Composer` | `session` | Per-session drafts (text and attachments); attachment chips above the text (icon or thumbnail, name, size, remove); Send with text, files or both; Stop while answering |
| `SetupScreen` | none | Download the pinned CLI, then sign in |
| `SettingsDialog` | none | Quick tasks (provider, model, auto-summary; saved on change), and the CLI version at the user's own risk |

## Keyboard

| Keys | Where | Does |
|---|---|---|
| Escape | Dialogs, popovers | Closes the topmost layer only |
| Enter / Shift+Enter | Composer | Send / new line |
| ← / → | Session (not while typing) | Previous / next page |
| B | Session (not while typing) | Toggle bookmark on the current page |
| O | Session (not while typing) | Show or hide the page outline |
| ← / → | A focused resize handle (sidebar, outline) | Move the edge left / right (Shift for bigger steps) |
| Ctrl+N (⌘N) | Anywhere once set up | New session |
| Ctrl+K (⌘K) | Anywhere once set up | Focus the search box; ↑ / ↓ move through results, Enter opens one, Escape clears the box (and again leaves it) |
| Enter | New-session dialog body | Start the session, or first open the folder picker if Co-work/Code has no folder (focus then moves to Start session). Buttons keep their own Enter |
| ← / → / ↑ / ↓ | New-session dialog, on the type cards | Choose the session type |
| Alt+↑ / Alt+↓ | A focused session in the sidebar | Move it up / down the list |
| Enter | Rename field | Save; Escape cancels |

## Running the UI without the app

`npm --prefix app/frontend run dev` (or `vite preview` after a build) runs
the UI on the mock backend. `?mock=setup`, `?mock=signin` and `?mock=empty`
pick a starting scenario; `?theme=light` or `?theme=dark` forces a theme
for screenshots.

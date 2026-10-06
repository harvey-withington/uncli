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
| `--cmd-sep` / `--cmd-read` / `--cmd-safe` / `--cmd-judged` / `--cmd-unsafe` / `--cmd-blocked` / `--cmd-rest` | A command on an approval card: separators, then each part's class words by what UNCLI makes of it (reads, safe, judged by the decision model, unsafe, blocked), and everything else. Aliases of `--kind-example`, `--kind-overview`, `--success`, `--accent`, `--warning`, `--danger`, `--text-faint`, so they follow the theme |
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
| `ResizeHandle` | `edge`, `width`, `min`, `max`, `label`, `onresize`, `oncommit` | A separator on a panel edge: drag, or arrow keys when focused (Shift for bigger steps). Used by the sidebar (200–440 px) and the side panel (240–960 px, one width for both tabs); widths kept in localStorage. Every scroller is positioned (`position: relative`) so absolutely positioned descendants such as `.visually-hidden` labels are clipped there, and `html` never scrolls |
| `ApprovalCard` | `session`, `approval` | A tool use waiting for the user, above the page controls (`role=alertdialog`; the panel fades out beneath them only while more of it is below): what Claude wants to do in plain words (`lib/approvals.ts`), a command through `CommandView`; one muted line on why when the colours can't say it (`cardNotes`: what an unsafe part could do, the decision model's reason, that UNCLI can't tell, or that the user asked to be prompted; "You marked this unsafe" is left to the colours). Buttons: Allow once, **This is safe** as a toggle (`aria-pressed`, green when on) with a Scope select (starts at the scope last used; fixed while on), and Deny. On, the parts' classes are on the safe list and the card waits for Allow once or Deny ("Marked as safe: … Click This is safe again to undo."); off, the list is as it was (`markSafe`). Off, the line under the buttons names what would be remembered; when nothing can be learned, the reason instead (`fixedReason`). No This is safe under Always. Never takes focus: the user may be typing |
| `CommandView` | `command`, `why?`, `shell`, `note?` | A command on a card: a shell command with reasons coloured part by part, one part a line (`commandLines`: separators, each part's class words and risk flags in its tone with the reason as a tooltip, the rest quiet). Long quoted text is shortened (`shortCommand`); past four lines the box scrolls. Under the box, Claude's description (`note`) on the left and "Show the whole command" / "Show it shorter" right-aligned opposite it, clear of the scrollbar |
| `SessionTypeAllowlist` | `sessionId`, `typeLabel` | "Safe in {type} sessions (built in)": the profile's allowlist as read-only pills (`allowLabel`: "npm test …", "Write in ./artifacts/**"; the raw entry in the tooltip), with a note that the user's Unsafe or Blocked entries still win |
| `CommandCheck` | `sessionId` | "Check a command": type a command, Enter or Check; shows Runs without asking / Claude prompts you first / Blocked (success, warning, danger) and the reason for each part. Runs nothing |
| `SafeList` | `sessionId`, `filter` | The safe list for the session's project and all projects: each entry's label if it has one (the user's own name for it, with the class faint beside it) or else its class in mono (a wrench for tools), a pencil to add or change the label in place (Enter saves, Escape cancels, empty clears), a verdict select (Safe / Unsafe in warning / Blocked in danger) and a scope select (moving to All projects promotes and merges), and remove. Below, Add: an example command or a tool name (a datalist of the session's known tools; the input has its own row), verdict, scope, Add; a preview list shows the class each part becomes, or why it can't be learned. Reloads in place, never blanking the list |
| `DecisionSection` | `prefs`, `savePrefs` | Settings → "Decision model" (`#settings-decider`): the quick-task model (saved on choosing) or a System One server, whose address, model, version, threshold and local-only are edited together and saved with Save (disabled until something changed); the API key in a password field with Save key / Remove key, never read back (the placeholder says whether one is saved); Test asks about a sample command and shows the verdict, how sure (calibrated deciders only), the time and the decider, as a `status` |
| `SafeSection` | `savePrefs` | Settings → "Safe and unsafe" (`#settings-safe`): what the list is, filter chips (Everything / This project / All projects / Built in, a radio group), `SafeList`, `SessionTypeAllowlist`, `CommandCheck`, then "When UNCLI can't tell" (the model decides / safe if it stays in this folder / prompt me). Takes plain values from the current session so its updates don't reload the list |
| `ReasonList` | `reasons`, `label`, `sessionId?` | The reason for each part of a command (or a tool use), as a list: the part in mono, then its reason, coloured by `reasonTone`: only read (faint), ran as safe (muted), judged by the decision model (accent, with a left bar, whichever way it went), prompted (warning, left bar), blocked (danger, left bar). With `sessionId`, each part UNCLI judged safe (`promptable`) has its own `ShouldPrompt` at the end of its line. Used by `TraceStrip` and `CommandCheck` |
| `ShouldPrompt` | `sessionId`, `classes` | At the end of one part's line in `ReasonList`: "This should prompt" (hand) with the Scope select; teaches that part's class Unsafe, then says "Claude will prompt before {class} from now on" |
| `ModeSwitch` | `session` | "Prompt me:" then a segmented radio group: Always (hand), When unsafe (shield), Never (zap); one tab stop, arrow keys move and wrap; Always selected in the accent, Never in the warning tint. Applies to the next tool use and to cards already waiting |
| `SessionStatus` | `session` | Status lines under the session's folder: the prompt level, always one line (When unsafe muted, Always accent, Never warning), then Unattended (warning) |
| `SearchPanel` | `results?` | The search box under New session (Ctrl+K) with filters for type, bookmarked, this session and date; while it has text, the sidebar shows grouped results (`results` mode) instead of sessions. Snippets come with \x01/\x02 markers, never HTML. Opening a result selects the page and scrolls to the first matching block, which flashes (`.search-flash`) |
| `Sidebar` / `SessionItem` | — / `session`, `active` | Drag to reorder (or Alt+↑ / Alt+↓); type icon and active stripe in the type's colour; double-click or pencil renames; trash confirms then deletes |
| `NewSessionDialog` | none | Profile cards, folder picker for co-work and code, model select. Opens on the welcome card clicked, else the last type used; each type starts from its last model and folder (remembered in the app database) |
| `SessionPane` | `session` | Header, toolbar, current page, nav bar, composer. The header's right side: `ModeSwitch`, then the Unattended toggle (coffee icon, `aria-pressed`, warning tint when on), which declines whatever would prompt the user, then the shield ("Safe and unsafe", opens Settings there), and the side panel toggle (`SidePanel`). `SessionStatus` lines sit under the folder. Requests the quick-task model is still judging (`judging`) get no card; a quiet "Checking whether a command is safe to run…" line shows in their place |
| `Toolbar` | `session` | Rendered from `toolbar.yaml`: model picker, modifier toggles (groups exclusive), slash and native items |
| `UsagePopover` | `usage`, `onclose` | Subscription windows from `EvUsageLimit` |
| `PageView` | `page` | Sticky question with copy, the files sent with it (`q-files`), streamed answer, banners, the files the turn changed (`FilesStrip`), trace, chips. A chevron collapses the question to compact mode: one line, the question left (ellipsis when short of room) and the page number right, never cut, then copy and the chevron; kept in localStorage |
| `AnswerBlocks` | `markdown`, `streaming?`, `markers?` | One block per top-level markdown element; each copies its source markdown. `markers` (block index → outline entry, from `app.outlineFor(page)`) puts the entry's section-kind icon in the page's left margin beside the block, 16 px in the kind colour (a step up from the outline's 13 px), so the margin always matches what the outline shows, Summary/Headings switch included |
| `CodeBlock` | `code`, `lang`, `streaming?` | Shiki highlighting once complete; copies code only |
| `SidePanel` | `page`, `scroller` | The panel beside the answer, as tabs (`role=tablist`, one tab stop, ← / → / Home / End move): "On this page" and, for session types with `artifacts`, "Artifacts", each with a count badge (the sections the outline shows, the artifacts as of the page; the badge's label reads "2 sections"). The aside is named after the tab showing. One width for both tabs (240–960 px, 360 to start), so switching tabs never moves the answer column; resized by the left edge (`ResizeHandle`, "Resize the side panel"); open state, tab and width kept in localStorage. The header's panel button opens or closes it on the tab it had; O and A open it on their tab, or close it when that tab is showing; View on a changed file opens the Artifacts tab on it |
| `OutlinePanel` | `page`, `scroller` | The "On this page" tab of `SidePanel`: the answer's headings (levels normalised; paragraphs that act as headings, such as "**1. Point.** …" or a short all-bold line, count one level below the real heading before them), or a summary from the quick-task model (sparkles button; Summary/Headings switch when both exist; caption names the model and cost; summaries also start on their own as answers finish, per the Summaries setting, and an answer skipped as too short says so with a link to summarise it anyway), each entry with a section-kind icon (`lib/sections.ts`: guessed for headings, chosen by the model in a summary; the kind is in the tooltip and read out to screen readers), the question at the top; click to jump below the sticky question, the section being read is highlighted. A faint caption says Headings or Summary when there is no switch |
| `ArtifactPane` | `page` | The Artifacts tab of `SidePanel`: "after page N", then the session's artifacts as the page shown left them (`artifactsAt` in `lib/artifacts.ts`; paging back pages them back), plus on the latest page the files in the folder no turn recorded; the ones this page changed carry a "this page" pill. The chosen one (per session, else what this page changed, else the first) opens in `ArtifactViewer` below the list |
| `ArtifactViewer` | `sessionId`, `entry` | One artifact, by kind: HTML in an iframe with `sandbox="allow-scripts"` (opaque origin) and `sandboxDoc`'s no-network policy; SVG and Mermaid (mermaid loaded on first use, strict level) in an iframe with an empty sandbox; Markdown through `AnswerBlocks`; images; text through `CodeBlock`; anything else a note. HTML, SVG and diagrams sit on white, as made for a page. The bar: name, which version ("as page N left it" or "in the folder now"), copy source, Open the current file (document types) and Show in folder |
| `TraceStrip` | `items`, `sessionId` | Collapsed summary with running, denied and failed counts. Each row says who let the call run (`approvedLabel`: "ran: safe", "ran: your safe list", "ran: only reads, safe" for a mixed command); when the row has reasons (`why`), that label, or "why" on a refused call, opens the reason for each part of the command (`ReasonList`). Takes `sessionId` for teaching |
| `FilesStrip` | `files`, `sessionId` | Under the answer, above the trace: "N files changed", open while the list is six or fewer. One row per file (`lib/files.ts`): an icon for how it changed (created in success, edited in the accent, changed by a command muted, deleted in danger, struck through; the words in the tooltip and read out), the path relative to the session folder (folders faint, the name in full, cut from the left when long), the edit's line, and the lines the turn's tools added and removed ("+12 −3" in success and danger, zero parts left out; none for a command's change, which UNCLI has no before and after for). The name is a button that does the row's first action (opens an artifact on the Artifacts tab, else Open). On hover or focus: Show in the artifact pane (layers; files in the artifacts folder of a session type with artifacts; opens the side panel's Artifacts tab on it), Open ("Open in {editor}" in sessions that link to an IDE, else Open with the default app, only for document types) and Show in folder; a deleted file has neither |
| `PageChips` | `page` | Model, modifiers, tokens, cost, duration |
| `NavBar` | none | Separate rounded buttons (gradient fill, filled icons, no text): previous bookmark ◀◀, back ◀, position capsule "3 / 7" (read as "Page 3 of 7"), forward ▶, next bookmark ▶▶, bookmark toggle (filled when set). Docked by `SessionPane` on the line between answer and composer, centred over the answer column; the answer fades out above it |
| `Icon` | `name`, `size?`, `label?`, `spin?`, `fill?`, `flip?` | `fill` for solid shapes, `flip` to mirror (a left triangle is a flipped play) |
| `Composer` | `session` | Per-session drafts (text and attachments); attachment chips above the text (icon or thumbnail, name, size, remove); Send with text, files or both; Stop while answering |
| `SetupScreen` | none | Download the pinned CLI, then sign in |
| `NotifySection` | `prefs`, `savePrefs` | Settings → "Notifications" (`#settings-notify`): which desktop notifications to show when a session the user isn't looking at finishes or needs approval (all / only approvals / off; saved on change), and Show a test (disabled while off). The app shows them natively (`internal/notify`); clicking one brings UNCLI forward on that session's latest page (`notifyOpen`) |
| `EditorSection` | `prefs`, `savePrefs` | Settings → "Editor" (`#settings-editor`): Automatic (names the first editor installed), each preset ("(not installed)" when it isn't on PATH; looked for again when Settings opens) or My own command…, which shows a command field with Save ({file}, {line}, {folder}); choosing a preset saves at once |
| `SettingsDialog` | none | Quick tasks (provider, model, auto-summary; saved on change), `NotifySection`, `EditorSection`, `SafeSection`, and the CLI version at the user's own risk. Opened from the sidebar, or at "Safe and unsafe" from the header's shield (`app.settingsAt = 'safe'`) |

## Keyboard

| Keys | Where | Does |
|---|---|---|
| Escape | Dialogs, popovers | Closes the topmost layer only |
| Enter / Shift+Enter | Composer | Send / new line |
| ← / → | Session (not while typing) | Previous / next page |
| B | Session (not while typing) | Toggle bookmark on the current page |
| O | Session (not while typing) | Open the side panel on On this page, or close it if that tab is showing |
| A | Session with artifacts (not while typing) | Open the side panel on Artifacts, or close it if that tab is showing |
| ← / → / Home / End | The side panel's tabs | Switch tab |
| ← / → | A focused resize handle (sidebar, outline) | Move the edge left / right (Shift for bigger steps) |
| Ctrl+N (⌘N) | Anywhere once set up | New session |
| Ctrl+K (⌘K) | Anywhere once set up | Focus the search box; ↑ / ↓ move through results, Enter opens one, Escape clears the box (and again leaves it) |
| Enter | New-session dialog body | Start the session, or first open the folder picker if Co-work/Code has no folder (focus then moves to Start session). Buttons keep their own Enter |
| ← / → / ↑ / ↓ | New-session dialog, on the type cards | Choose the session type |
| Alt+↑ / Alt+↓ | A focused session in the sidebar | Move it up / down the list |
| Enter | Rename field | Save; Escape cancels |

## Running the UI without the app

`npm --prefix app/frontend run dev` (or `vite preview` after a build) runs
the UI on the mock backend. `?mock=setup`, `?mock=signin`, `?mock=empty`,
`?mock=unattended`, `?mock=readonly` and `?mock=full` pick a starting
scenario (in the mock, a message mentioning "push" asks to run git push, and
one mentioning a "note" asks to use an MCP tool with no annotations; the Chat session has two pages of artifacts: HTML, SVG, Mermaid and a Markdown file only in the folder); `?theme=light` or `?theme=dark` forces a theme
for screenshots.

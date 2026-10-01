# UNCLI — Phased Build Brief

Oct 1, 2026 · @Harvey Poppington

## Purpose and scope

UNCLI (pronounced "un-clee") is a desktop GUI layer over AI CLIs, starting with Claude Code, so you can run many Claude sessions without a terminal. The name reads as "un-CLI", the command line without the command line, and sits beside BRUV as a family. Domain: uncli.app. In code, packages and binaries use lowercase `uncli`.

**What UNCLI is:** a multi-session manager, a paged conversation reader, an approval surface for tool permissions, and a launcher into the user's real tools.

**What UNCLI is not:** an IDE, editor, diff viewer or git client. When work turns into coding, UNCLI links into the user's IDE at the right file and line, and stops there.

**Backend rule: pure CLI.** UNCLI never calls a model API directly. Every session is a long-lived CLI process driven over stdin/stdout. This keeps subscription billing, gives one code path, and makes other providers (Gemini CLI, Codex CLI) a matter of adding adapters.

**Design rule: one of each, seams for all.** Phase 1 ships exactly one implementation of each core interface (one adapter, one runtime, three built-in profiles). Every later feature in this brief must land as a new implementation or a new config file, never as a rewrite of the core. If a phase needs a core change, the architecture section was wrong; fix the brief first.

**Modes:** Chat (no project, scratch folder, artifacts), Co-work (a user folder of documents, artifacts) and Code (a repo, full tools, IDE links). Modes are profiles, not code paths.

## Stack and repo layout

Go + Wails + Svelte 5 + TypeScript, the same stack as BRUV, with SQLite for app state. Use Wails v2 unless v3 is stable at build time; keep Wails-specific code in one package so the switch is cheap.

| Layer | Choice | Why |
| --- | --- | --- |
| Backend | Go 1.23+ | Subprocess and stream handling, fsnotify, Docker SDK later |
| Shell | Wails | Native window, Go-to-JS bindings and events |
| UI | Svelte 5 + TypeScript | Runes for streaming state; shared look with BRUV |
| Markdown | markdown-it + Shiki | Source line maps (copy original markdown), good highlighting |
| Storage | SQLite via modernc.org/sqlite | Pure Go, no cgo, cross-compiles cleanly |
| Config | YAML files | Profiles and modifiers are user-editable data |
| Icons | Lucide | Matches the toolbar and modifier icon names |

```
uncli/
  main.go                 # Wails bootstrap only
  internal/
    core/                 # interfaces + normalized events (no deps on anything else)
    adapter/claude/       # Claude Code CLI: arg builder + stream-json parser
    runtime/local/        # local process runtime
    runtime/container/    # (phase 5)
    session/              # session lifecycle, turn assembly, state machine
    store/                # SQLite: sessions, pages, bookmarks, usage
    profile/              # profile + modifier loading and resolution
    permission/           # local MCP server for --permission-prompt-tool
    watch/                # fsnotify: artifacts dir, touched files
    ide/                  # open-in-editor launchers
    bridge/               # the ONLY package that imports Wails
  config/defaults/        # built-in profiles.yaml, modifiers.yaml, toolbar.yaml
  frontend/src/
    lib/api/              # typed wrapper over bridge calls + event subscription
    lib/render/           # markdown-it setup, copy-block plugin, Shiki
    components/           # Sidebar, PageView, NavBar, Toolbar, ApprovalCard, ArtifactPane
    stores/               # sessions, pages, activity
  testdata/streams/       # recorded CLI stream fixtures (*.jsonl)
```

The rule that keeps the kitchen sink possible: `core` imports nothing internal, and only `bridge` imports Wails. If UNCLI ever runs as a local server for a docked or remote UI, `bridge` is the only package replaced.

## Architecture

A session is an adapter (which CLI) running on a runtime (where it runs), configured by a profile plus modifiers, emitting normalized events that the session assembles into pages. The UI only ever sees UNCLI's own events and pages, never a provider's raw output.

&#91;embedded content: UNCLI architecture · one session's components\]

Everything above the CLI process is UNCLI's; swapping the adapter or runtime box is how phases 5 and 6 land.

### Core interfaces (`internal/core`)

```go
// Adapter: one per CLI provider. Phase 1: claude.
type Adapter interface {
    ID() string
    Capabilities() Capabilities
    BuildCommand(spec LaunchSpec) (Command, error) // binary, args, env
    EncodeTurn(t UserTurn) ([]byte, error)         // one stdin line
    EncodeControl(c Control) ([]byte, bool)        // interrupt, set model; false = unsupported
    NewParser() Parser                             // stateful, one per process
}

type Parser interface {
    Feed(line []byte) ([]Event, error) // unknown event types -> EvUnknown, never an error
}

// Runtime: where the process lives. Phase 1: local. Phase 5: container.
type Runtime interface {
    ID() string
    Start(ctx context.Context, cmd Command, workdir string) (Proc, error)
}

type Proc interface {
    Stdin() io.Writer
    Stdout() io.Reader
    Stderr() io.Reader
    Wait() error
    Kill() error
}

// LaunchSpec: everything resolved from profile + model + system-scope modifiers.
type LaunchSpec struct {
    ResumeID        string            // provider session id; empty = new
    Model           string
    Workdir         string
    SystemPrompt    string            // replace (chat profile)
    AppendPrompt    string            // append (code, co-work)
    AllowedTools    []string
    DisallowedTools []string
    PermissionMCP   *PermissionConfig // phase 2
    Settings        map[string]any    // thinking budget etc.
    Env             map[string]string
}

type UserTurn struct {
    Text        string
    Directives  string       // rendered turn-scope modifiers, hidden in the UI
    Attachments []Attachment // images, phase 3
}
```

### Components

**Profile resolver** (`profile`). Merges built-in YAML with user overrides by `id`. Takes a profile, a model and the active modifiers; returns a `LaunchSpec` (from system-scope modifiers) and a directive renderer (for turn-scope modifiers). Per-model modifier text is matched by family glob (`opus*`), falling back to `default`.

**Session** (`session`). Owns one process at a time. Runs a reader goroutine that feeds stdout lines to the parser, updates the activity state machine, assembles pages and writes them to the store. Model switches and system-scope modifier changes trigger a **respawn**: wait for idle (or queue the change), kill, relaunch with `ResumeID`. If the adapter reports `LiveModelSwitch`, send a control message instead.

**Store** (`store`). SQLite for sessions, pages, bookmarks, usage and the raw event log. The provider's own transcript remains the source of truth for conversation content; UNCLI's store holds structure, metadata and a replayable copy.

**Bridge** (`bridge`). The only Wails importer. Exposes bound methods (create session, send turn, switch model, toggle modifier, answer approval, open in IDE) and emits per-session events to the frontend, with text deltas coalesced to about every 50 ms.

**Permission bridge** (`permission`, phase 2). One local MCP server on 127.0.0.1, started with the app. Each session's generated `--mcp-config` points at it with a per-session token, and `--permission-prompt-tool` names its `approve` tool. It rejects any request carrying a browser Origin header or lacking its exact JSON content type, so a web page can never reach it. A call blocks until the UI answers or it times out, and the session shows the Needs approval state meanwhile.

**Watchers** (`watch`). fsnotify on each session's artifacts folder (chat and co-work) and, for code sessions, the repo (debounced, ignore-listed). Watchers feed the artifact pane and the touched-files list.

**IDE launcher** (`ide`). Configurable command templates such as `code -g {file}:{line}`, with presets for VS Code, Cursor and Antigravity.

### Data flow for one turn

1. UI calls `SendTurn(sessionID, text)`. The session renders the directives, creates a page row (question plus active model and modifiers) and writes `EncodeTurn` to stdin.
2. The reader goroutine parses stdout into events. Each event goes to the raw log, updates the activity state and is appended to the open page.
3. The bridge emits events to the UI, coalescing deltas.
4. A `TurnResult` event closes the page, records usage and sets Unread if the session isn't focused.

## Events and capabilities

The normalized event model is the most important contract in UNCLI: get it right in phase 1 and every later adapter, runtime and UI feature plugs into it.

```go
type EventKind string

const (
    EvSessionReady  EventKind = "session_ready"   // provider session id, model, tools
    EvTurnStarted   EventKind = "turn_started"
    EvThinking      EventKind = "thinking"        // optional text
    EvTextDelta     EventKind = "text_delta"
    EvTextBlock     EventKind = "text_block"      // a completed assistant text block
    EvToolStarted   EventKind = "tool_started"    // id, name, input
    EvToolFinished  EventKind = "tool_finished"   // id, ok, summary, output (truncated)
    EvFileTouched   EventKind = "file_touched"    // path, line, how (edit/write/bash-detected)
    EvApprovalAsked EventKind = "approval_asked"  // request id, tool, input (phase 2)
    EvTurnResult    EventKind = "turn_result"     // usage, cost, duration, is_error
    EvError         EventKind = "error"
    EvExited        EventKind = "exited"          // exit code
    EvUnknown       EventKind = "unknown"         // raw line kept, never dropped
)

type Event struct {
    Kind    EventKind
    TurnSeq int             // page number within the session
    At      time.Time
    Data    json.RawMessage // kind-specific payload
    Raw     json.RawMessage // original provider line, for the log and fixtures
}
```

`EvFileTouched` is derived by the adapter from Edit, MultiEdit and Write tool inputs, and by the repo watcher for changes made through Bash. Both feed the same touched-files list.

### Activity state machine

| State | Entered on | Left on |
| --- | --- | --- |
| Starting | process spawned | `session_ready` |
| Idle | `session_ready`, or `turn_result` while focused | turn sent |
| Thinking | turn sent, or `thinking` | first text or tool event |
| Writing | `text_delta` / `text_block` | tool started or result |
| Running tools | `tool_started` with no matching finish | all tools finished |
| Needs approval | `approval_asked` | user answers |
| Unread | `turn_result` while unfocused | session focused |
| Error / Exited | `error` with is\_error, or `exited` | respawn |

### Capabilities

Each adapter declares what it supports. The UI hides or greys out controls the current adapter can't back, so a weaker CLI plugs in without special cases.

```go
type Capabilities struct {
    PartialStreaming bool // token deltas
    Resume           bool
    LiveModelSwitch  bool // control message instead of respawn
    Interrupt        bool
    Approvals        bool // permission prompt routing
    Images           bool
    UsageReporting   bool
    ThinkingEvents   bool
    SlashPassthrough bool
}
```

## Data model and config

App state lives in one SQLite file in the user config dir; profiles, modifiers and toolbar are YAML, with built-ins embedded in the binary and user files overriding them by `id`. Columns for later phases exist from day one so no migration is needed to add them.

### SQLite schema

```sql
CREATE TABLE sessions (
  id            TEXT PRIMARY KEY,   -- uncli uuid
  title         TEXT,
  adapter       TEXT NOT NULL,      -- 'claude'
  runtime       TEXT NOT NULL,      -- 'local' | 'container'
  runtime_ref   TEXT,               -- container profile id (phase 5)
  profile_id    TEXT NOT NULL,      -- 'chat' | 'cowork' | 'code' | user ids
  workdir       TEXT NOT NULL,
  provider_sid  TEXT,               -- CLI session id, for --resume
  model         TEXT NOT NULL,
  modifiers     TEXT NOT NULL DEFAULT '[]',  -- active modifier ids, JSON
  sort_order    REAL,
  archived      INTEGER DEFAULT 0,
  created_at    INTEGER, updated_at INTEGER
);

CREATE TABLE pages (
  id            TEXT PRIMARY KEY,
  session_id    TEXT REFERENCES sessions(id),
  seq           INTEGER NOT NULL,   -- 1-based page number
  question      TEXT NOT NULL,      -- as typed, without directives
  directives    TEXT,               -- what was injected, for the chips
  model         TEXT NOT NULL,
  modifiers     TEXT NOT NULL,
  answer_md     TEXT,               -- final assistant markdown
  trace         TEXT,               -- JSON: tool calls, summaries
  touched_files TEXT,               -- JSON: [{path, line, how}]
  artifacts     TEXT,               -- JSON: [{path, commit}]
  status        TEXT,               -- open | done | error | interrupted
  bookmarked    INTEGER DEFAULT 0,
  pinned        INTEGER DEFAULT 0,
  input_tokens  INTEGER, output_tokens INTEGER,
  cache_read    INTEGER, cache_write INTEGER,
  cost_usd      REAL, duration_ms INTEGER,
  started_at    INTEGER, finished_at INTEGER,
  UNIQUE(session_id, seq)
);

CREATE TABLE events (            -- raw log: replay, debugging, fixture capture
  session_id TEXT, page_seq INTEGER, n INTEGER,
  kind TEXT, at INTEGER, data TEXT, raw TEXT,
  PRIMARY KEY (session_id, page_seq, n)
);

CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT);
```

The `events` table can grow large. Keep it, but cap it per session (configurable) and prune oldest pages first.

### Profiles (`profiles.yaml`)

```yaml
profiles:
  - id: chat
    label: Chat
    icon: message-circle
    folder: scratch          # scratch | pick | repo
    model: sonnet
    system_prompt: |         # replaces the coding persona
      You are a helpful assistant in a desktop chat app. Put any substantial
      output (HTML, SVG, Mermaid, long documents) in a file under ./artifacts/.
    allowed_tools: [WebSearch, WebFetch, "Write(./artifacts/**)"]
    permission_mode: default
    modifiers_on: []
  - id: cowork
    label: Co-work
    icon: briefcase
    folder: pick
    model: sonnet
    append_prompt: |
      You are helping with documents and knowledge work in this folder.
      Put generated deliverables under ./artifacts/.
    allowed_tools: [Read, Write, Edit, Glob, Grep, WebSearch, WebFetch]
  - id: code
    label: Code
    icon: code
    folder: repo
    model: opus
    allowed_tools: []        # empty = CLI defaults
    ide_links: true
```

### Modifiers (`modifiers.yaml`)

```yaml
modifiers:
  - id: use-agents
    label: Use Agents
    icon: users
    scope: turn              # turn = directive block; system = respawn
    requires_tools: [Task]
    text:
      default: "Delegate to subagents wherever work can be parallelised or isolated."
      "haiku*": "Use subagents for independent subtasks. Keep each delegation brief."
  - id: efficiency
    label: Efficiency Mode
    icon: gauge
    scope: turn
    group: verbosity         # mutually exclusive within a group
    text:
      default: "Use the minimum tokens and turns needed. No preamble, no recap."
  - id: thorough
    label: Thorough
    icon: microscope
    scope: turn
    group: verbosity
    settings: { thinking: high }
    text:
      default: "Verify, check edge cases, and explain trade-offs."
```

The directive block restates the full active set every turn, wrapped in `<session_directives>`. When a modifier was switched off since the previous turn, add one line saying its earlier instruction no longer applies.

### Toolbar (`toolbar.yaml`)

```yaml
toolbar:
  - { kind: model_picker }
  - { kind: modifiers }                       # renders all modifiers as toggles
  - { kind: separator }
  - { kind: slash, label: Compact, icon: shrink, command: /compact }
  - { kind: native, label: Usage, icon: bar-chart, action: usage }
  - { kind: native, label: Open folder, icon: folder-open, action: open_workdir }
```

`slash` items are sent into the session as text. `native` items are implemented by UNCLI itself, because many interactive slash commands don't work in headless mode.

## Phase 1: weekend MVP

By Sunday night UNCLI runs several Claude sessions side by side in a paged, bookmarkable UI, with model switching and modifier toggles, and never shows a terminal. Approvals, artifacts and IDE links wait for phase 2; phase 1 avoids approvals by giving each profile a pre-approved tool list.

### In scope

- Claude adapter (stream-json in and out, partial messages) with fixture-based parser tests.
- Local runtime; session manager; resume across app restarts via `--resume`.
- SQLite store for sessions, pages, bookmarks and the raw event log.
- Sidebar: session list with a mode icon, title and activity indicator; new-session dialog (profile, folder, model).
- Page view: pinned question, streamed markdown answer, collapsed tool trace strip, per-page chips (model, modifiers, tokens).
- Nav bar: previous bookmark, back, forward, next bookmark, toggle bookmark. Previous and next bookmark fall back to the first and last page when no bookmark lies in that direction. Keyboard: left/right arrows, `B` to bookmark.
- Copy on hover for the question, every code block and every markdown block (copies source markdown).
- Model picker that respawns with resume.
- Turn-scope modifiers from `modifiers.yaml`, with groups and per-model text.
- Three built-in profiles; a toolbar rendered from `toolbar.yaml` (model picker, modifiers, two slash commands).
- Interrupt (stop the current turn) if the CLI supports it; otherwise kill and respawn.

### Out of scope (deliberately)

Approval UI, artifact pane, touched files and IDE links, images, pins, search, usage dashboard, profile and modifier editors, containers, other providers.

### Phase 1 permissions

Chat and co-work run with their tool allowlists from the profile. Code runs with `--permission-mode acceptEdits` and an allowlist of safe Bash patterns from the profile (for example `Bash(git status:*)`, `Bash(npm test:*)`). Anything else is denied by the CLI, and the denial appears in the trace strip. Never use the bypass mode as a default.

### Build order

| # | Task | Est. |
| --- | --- | --- |
| 1 | **Spike:** run the CLI by hand in stream-json mode, multi-turn, with resume and an interrupt; save outputs to `testdata/streams/` | 2 h |
| 2 | Scaffold Wails + Svelte; `core` types; `bridge` skeleton | 1 h |
| 3 | Claude adapter: command builder, turn encoder, parser; tests over fixtures | 3 h |
| 4 | Local runtime + session (reader goroutine, state machine, page assembly) | 3 h |
| 5 | SQLite store + resume on startup | 2 h |
| 6 | Profile and modifier loading; directive rendering | 1.5 h |
| 7 | Sidebar + new-session dialog | 2 h |
| 8 | Page view, markdown renderer with copy plugin, Shiki, trace strip | 4 h |
| 9 | Nav bar + bookmarks + keyboard | 1.5 h |
| 10 | Toolbar, model picker (respawn), modifier toggles | 2 h |
| 11 | Visual polish pass: theme tokens, light and dark, motion | 2 h |
|  | **Total** | **24 h** |

That total is tight for a weekend. If it slips, cut in this order: polish pass to an hour, keyboard shortcuts, interrupt, the modifier groups.

### Definition of done

- [ ] Three sessions (one per profile) run at once, each with a correct live activity state
- [ ] Quitting and relaunching restores every session, its pages and bookmarks, and continues the conversation
- [ ] Switching model mid-session continues the same conversation; the page chips show the switch
- [ ] Toggling Efficiency Mode changes the next answer and its chips; switching it off is honoured
- [ ] Every code block, markdown block and question copies correctly
- [ ] Parser tests pass on all recorded fixtures, and an unknown event type doesn't crash anything
- [ ] No terminal window appears at any point on Windows or macOS

## Phases 2 to 6

Each phase is a new implementation behind an existing seam; none of them changes `core` beyond adding event kinds or capability flags.

### Phase 2: daily driver

The goal is that UNCLI replaces the terminal for real work.

- **Permission bridge:** local MCP server (per-session token, browser Origin headers rejected), generated `--mcp-config`, `--permission-prompt-tool`. Approval cards with Allow, Deny and Always for this session. Session-level allow rules stored on the session row. Code profile drops the narrow Bash allowlist.
- **Artifact pane:** watcher on `./artifacts/`, sandboxed iframe renderer with strict CSP for HTML, SVG and Mermaid, markdown viewer. The artifacts folder is a git repo, auto-committed at turn end; each page links to the artifact versions it produced, so paging back pages the artifact back too.
- **Touched files + IDE links:** `EvFileTouched` from tool inputs plus the repo watcher; a per-page list with open-in-IDE buttons using the `ide` presets.
- **Desktop notifications** when a background session finishes or needs approval.

### Phase 3: comfort

- Pins (separate from bookmarks), page search across sessions, session rename and archive, drag to reorder.
- **Usage dashboard** from page usage columns: per session, day, model and profile.
- **Images:** drag and drop or paste into the composer, sent as content blocks (if `Images` capability).
- **Import:** read existing `~/.claude/projects/*.jsonl` transcripts into UNCLI sessions so terminal sessions can be continued in the GUI.
- Theme tokens shared with BRUV; command palette; full keyboard map.

### Phase 4: customisation

- Editors for user profiles, modifiers (with per-model variants) and the toolbar; user files override built-ins by `id`.
- **System-scope modifiers** (long personas or house styles), applied by respawn.
- `LiveModelSwitch` via the control protocol if the CLI exposes it, keeping respawn as fallback.
- Cost hint on the model picker: estimated tokens to re-read after a cache-invalidating switch.

### Phase 5: containers

- **Container runtime:** Docker Engine API from Go; `Start` runs `docker exec -i <ctr> claude ...` so the stdio transport is unchanged.
- **Container profiles** (`containers.yaml`): base image, packages, git, toolchains, MCP servers (for example BRUV over Tailscale), network policy. UNCLI compiles them into a cached Dockerfile layer.
- **Auth:** `CLAUDE_CODE_OAUTH_TOKEN` from `claude setup-token`, stored in the OS keychain, passed as env. Never mount credentials.
- **Brain modes:** Shared (skills, commands, agents and a memory directory mounted read-write; `settings.json` generated read-only), Sandboxed (a persistent volume per container profile) and Reviewed (an overlay whose changes UNCLI shows as a diff for approval before merging into `~/.claude`).
- Per-container transcript volume that UNCLI can read for resume and import.

```yaml
containers:
  - id: go-dev
    label: Go dev
    base: uncli/claude-base:latest
    packages: [git, make]
    toolchains: { go: "1.23" }
    mcp: [bruv-home]
    network: default          # default | none | allowlist
    brain: shared             # shared | sandboxed | reviewed
```

### Phase 6: other providers and reach

- **Adapters** for Gemini CLI and Codex CLI (headless JSON modes), each with its own fixtures and capability flags. Modifiers gain optional per-provider text and flag mappings.
- **Server mode:** replace `bridge` with an HTTP and WebSocket server so the same Svelte UI runs in a browser, a docked IDE webview or a tablet on the Tailscale network.
- Anything else that turns up, as an adapter, runtime, profile or modifier.

## Verify first, and risks

The phase 1 spike must confirm the CLI facts this brief assumes, because they were written from memory and the CLI changes quickly. Check each against the installed CLI and current docs before writing the adapter.

- [ ] `claude -p --input-format stream-json --output-format stream-json --verbose` stays alive for multiple turns over stdin
- [ ] `--include-partial-messages` produces text deltas, and their exact event shape
- [ ] `--resume <id>` works in stream-json mode, and where the session id appears in the init event
- [ ] `--model`, `--system-prompt`, `--append-system-prompt`, `--allowedTools`, `--disallowedTools` and `--permission-mode` names and behaviour
- [ ] How to interrupt a running turn (control message or signal) without losing the session
- [ ] The `result` event's usage and cost fields
- [ ] Which slash commands work when sent as text in headless mode
- [ ] Phase 2: `--permission-prompt-tool` and `--mcp-config` request and response shapes
- [ ] Phase 3: image content blocks on stream-json input
- [ ] Phase 4: whether a live model switch exists in the control protocol
- [ ] Phase 5: `claude setup-token` and `CLAUDE_CODE_OAUTH_TOKEN` behaviour inside a container

| Risk | Effect | Mitigation |
| --- | --- | --- |
| Stream-json format changes in a CLI update | Parser breaks, UI goes blank | Pin a known-good CLI version per profile; fixture tests; `EvUnknown` never drops a line |
| Headless gaps (slash commands, interrupts) | Toolbar buttons do nothing | `native` toolbar actions; capability flags hide what can't work |
| Per-turn token overhead of the CLI's system prompt and tools | Chat burns subscription limits faster | Chat profile replaces the system prompt and restricts tools |
| Cache miss on model switch | Expensive first turn after a switch | Cost hint on the picker (phase 4) |
| Container writes to host `~/.claude` hooks | Code runs on the host | `settings.json` generated read-only; credentials never mounted |
| Windows process quirks (console windows, child cleanup) | Flashing terminals, orphaned CLIs | `HideWindow` on Windows; job objects or process groups so children die with UNCLI |
| Using subscription tokens outside the official CLI | Terms of service breach | UNCLI only ever drives the official binary |

## Working rules for the builder

These rules are for the Claude session that builds UNCLI; copy this section into the repo's `CLAUDE.md`.

1. **Spike before code.** Do build-order task 1 first and update this brief with anything the CLI does differently. Don't write the adapter from assumptions.
2. **Fixtures are the contract.** Every new provider behaviour gets a recorded `.jsonl` fixture and a parser test before UI work depends on it.
3. **Respect the seams.** `core` imports nothing internal; only `bridge` imports Wails; the UI never parses provider JSON. A change that breaks a seam needs the brief updated first.
4. **Config over code.** Profiles, modifiers, toolbar and container profiles are YAML data. If a feature can be a config entry, it is one.
5. **Not an IDE.** No editors, diff viewers or git UIs. Anything code-shaped becomes a link into the user's IDE.
6. **Pure CLI.** No direct model API calls, and never reuse CLI credentials outside the official binary.
7. **Visual bar.** Aim for the UI people wish Anthropic had shipped: generous spacing, one accent colour, real typography, subtle motion on state changes, polished light and dark themes. No default component-library look.
8. **Phase discipline.** Finish a phase's definition of done before starting the next. Out-of-scope ideas go in a backlog list at the end of this brief, not into the current phase.

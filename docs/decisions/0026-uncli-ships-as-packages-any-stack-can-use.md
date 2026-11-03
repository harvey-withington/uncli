# 0026. UNCLI ships as packages any stack can use

Status: accepted
Date: 2026-11-02

## Context

UNCLI is meant to be embedded in other applications (decision 0010), with
BRUV as the first host. The direction was settled on 2026-10-20 and
2026-10-24:
- There is one Svelte UI, shipped framework-free as the `<uncli-chat>` web
  component.
- Thin React, Vue and Angular wrappers sit on top, with a vanilla
  TypeScript client underneath.
- The element takes a `Backend` from its host.
- Nothing in UNCLI names a host.

On 2026-11-02 Harvey added that the components should ideally be usable from
any stack, not just Svelte.

The frontend is one app today, and a survey of it on 2026-11-02 found:
- One `AppStore` serves the whole window. Its per-session state is already
  keyed by session id, but some actions and components act on "the current
  session".
- The panel and compact layout is one setting shared by every session.
- Toasts and the confirm dialog are module-level singletons.
- `app.css` puts the tokens on `:root` and relies on shared global classes.
- A handful of helpers assume `document` and `body`, which breaks inside a
  shadow root.
- The phone app reuses the chat components, and duplicates the session
  header.

## Decision

Harvey, 2026-11-02, after the design in the "Split UNCLI out into a reusable
component package" card:

- **Packages.** The packages live in npm workspaces under
  `app/frontend/packages/`, beside the app, which keeps building from
  `app/frontend`:
  - `@uncli/client`: the client, with `@uncli/client/mock`.
  - `@uncli/chat`: the Svelte chat and the elements.
  - `@uncli/react`, `@uncli/vue` and `@uncli/angular`: wrappers.

  The Wails backend stays in the app. Versions move with UNCLI's own.
- **Not published yet.** The packages stay in the workspace, and a host
  such as BRUV uses them through a local path or a packed tarball until
  their contract settles. Publishing (where, and under which scope) is
  decided before the first outside host. They carry UNCLI's MIT licence.
- **Plain TypeScript stores in the client.** `@uncli/client` has no Svelte
  dependency. It has three parts:
  - A `ChatClient` per backend owns the subscription, the shared reference
    data, the session list, the connection and resync.
  - A `SessionStore` per session owns that session's pages, live text,
    commands, artifacts, files, MCP links and summaries. Its actions need no
    session argument.
  - Both have `subscribe(listener)` and `snapshot()`, so any stack can
    observe them. React's `useSyncExternalStore` and Vue's refs plug in
    directly.

  A thin Svelte adapter turns them into runes for UNCLI's components. The
  desktop and phone shell keeps an `AppStore` of its own for the sidebar,
  search, dialogs, settings and pins.
- **The chat gets a shell and a layout from whatever holds it.**
  - `ChatShell` covers toasts, confirm, and opening Settings or the usage
    dashboard (these two are optional, so an embedded chat leaves them out),
    plus links.
  - `ChatLayout` covers the panel and compact state.

  Both are provided per root, which replaces the singletons.
- **Styles on UNCLI's root.** The tokens, dark rules, shared classes and
  focus styles key on UNCLI's root (`.uncli` in the app, `:host` in an
  element), not `:root`, as decision 0010 planned. The scheme attribute
  moves there too. Only the app imports the document rules (`html`, `body`,
  the fonts).
- **Two public elements.**
  - `<uncli-chat>`: one conversation.
  - `<uncli-session-list>`: a list of sessions.

  Elements on one page share a client per backend and one adopted
  stylesheet. Each element is a small class that attaches a shadow root and
  mounts the Svelte root component in it; it doesn't use Svelte's
  `customElement` compile. Smaller elements (a page view, a composer, an
  approvals list) wait for a host that needs them.
- **Generated wrappers.** The elements are described once in a Custom
  Elements Manifest (`custom-elements.json`). The React, Vue and Angular
  wrappers and their types are generated from it.
- **Fonts.** An element uses the host's font and the system monospace. A
  host that wants UNCLI's own look imports `@uncli/chat/fonts.css`. An
  element never writes to the host's document.
- **Layout memory.** An element starts from its attributes. It remembers
  its layout only when the host gives it a `storage-key`, under that key.
- **The app mounts `Chat.svelte` now and the element later.**
  - The desktop and phone apps mount `Chat.svelte`, the root component the
    element wraps. Its one header replaces both `SessionPane`'s and
    `MobileSession`'s.
  - Moving the app onto `<uncli-chat>` itself, so that daily use proves the
    element, is a later step with its own card.

## Consequences

- **The steps.** The work goes in steps that each leave the app as it was:
  0. a spike in a shadow root;
  1. the workspace and the client's API;
  2. per-root services and the CSS move;
  3. the store rewrite;
  4. `Chat.svelte`;
  5. the elements;
  6. the wrappers.

  The spike confirms the element approach and the bundle size. If it
  disproves either, a new record supersedes this one.
- **The store rewrite is the biggest and riskiest step.** The plain
  TypeScript stores replace the 809-line runes store instead of splitting
  it, so the resync logic and the store tests move first. The client gains
  tests that run without Svelte.
- **What's public.** Only what `custom-elements.json` and the host docs
  list, plus the `--uncli-*` tokens (decision 0010). Renaming any of these
  is a breaking change. Everything else stays internal and free to change.
- **No outside host yet.** Until the packages are published, no outside
  host can install them, and BRUV's build depends on a UNCLI checkout or a
  tarball.
- **Different fonts per host.** An embedded chat looks slightly different
  in each host's font, so UNCLI's layout must hold up in fonts it wasn't
  tuned for.
- **What we gave up.** Native per-framework components and one element per
  Svelte component are both ruled out: one would mean several UIs to
  maintain, the other a public contract for every internal component.

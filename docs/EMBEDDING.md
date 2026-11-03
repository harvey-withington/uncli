# Embedding UNCLI

UNCLI's chat can be shown in any web page as the `<uncli-chat>` element
(decision 0026). It's the same Svelte chat UNCLI's own app shows, shipped as
a standard custom element, so it works in any stack. This page is for host
authors. It covers what exists today and says what doesn't yet.

## Status

| | |
|---|---|
| **Status** | Alpha. It's built on the `feat/packages` branch and not published. |
| **Build** | `npm --prefix app/frontend run build:embed` writes the element to `app/frontend/dist-embed/lib/uncli-chat.js`, and a sample host page on UNCLI's mock to `dist-embed/sample/`. |
| **Client library** | `@uncli/client` (`app/frontend/packages/client`) is the library underneath. It isn't published either. |

## Showing a chat

```html
<uncli-chat session="s-code" scheme="dark" storage-key="card-chat"></uncli-chat>
<script type="module">
  import './uncli-chat.js' // registers <uncli-chat>
  import { httpBackend } from '@uncli/client'

  const chat = document.querySelector('uncli-chat')
  chat.backend = httpBackend({ base: 'https://pc.tailnet.ts.net:9871', token: () => deviceToken })
</script>
```

- **`backend`** (property) is how the chat reaches UNCLI.
  - `httpBackend` reaches a UNCLI over its remote access server (decision 0021), with a paired device's token.
  - Any object implementing `Backend` works, including `@uncli/client/mock`.
  - Every element given the same backend object shares one client: one event stream, one copy of the server's state.
- **`session`** (attribute or property) is the session to show. Changing it shows another.
- **`scheme`** is `light` or `dark`. Leave it out to follow the system.
- **`storage-key`** remembers the side panel and the compact layout under `uncli-layout:<key>` in the page's local storage. Without it the chat remembers nothing, so UNCLI never writes to your storage unasked.

The element fills the box you give it, so give it a height. In a grid or flex layout, let its container shrink (`min-width: 0`).

## Events

The element sends these events. They bubble and are composed, so you can listen on the element or the document. `detail` is `{ session }`.

| Event | When |
|---|---|
| `uncli-ready` | The chat has the server's state and shows its session |
| `uncli-busy` | The session started answering |
| `uncli-idle` | It finished |
| `uncli-needs-approval` | A tool call is waiting for the user |

## Looking like your app

The chat lives in a shadow root:
- Your page's styles don't reach it, and its styles don't reach your page.
- Two names can still be the same (`--bg`, `.btn`) without clashing.

You theme it through UNCLI's public tokens (decision 0010):
- Set any `--uncli-*` custom property on the element, usually as a reference to your own tokens: `uncli-chat { --uncli-accent: var(--brand); }`.
- The list of tokens is in `docs/UI-CONVENTIONS.md`.

**Fonts.**
- The chat uses your page's font, and the system monospace for code.
- To use UNCLI's own fonts (Inter and JetBrains Mono), load them in your page and set `--uncli-font` and `--uncli-mono`.
- The element doesn't load fonts or write anything into your document.

**Overlays.** Toasts and confirm dialogs stay inside the element.

## Size

The element is one ES module, with Svelte and UNCLI's client inside it. Code highlighting's languages load separately, only when a code block needs one.

- **Measured size:** about 215 KB gzipped, unminified, on 2026-11-03.
- **Load it lazily:** a dynamic `import()` when a chat first opens.

## What's not there yet

- **A compact layout.** The element shows the desktop session layout. It needs about 700 px of width, and narrower containers clip it. The phone's layout becomes the compact one when the desktop and phone session headers merge.
- **`<uncli-session-list>`,** the session list as its own element.
- **The host adapter.** Not yet available:
  - `openLink` for the host's own links;
  - `complete` for @mentions;
  - `turnContext`;
  - `t` for the host's strings.
- **Cancelable `uncli-escape` and `uncli-submit` events,** named slots (`header-extra`, `below-composer`), and `::part()` names.
- **React, Vue and Angular wrappers,** generated from a Custom Elements Manifest.
- **Host credentials and host sessions** (the host protocol). Until then, a host reaches UNCLI as a paired device.

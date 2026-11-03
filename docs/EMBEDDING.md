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
| **Packages** | The elements and the Svelte components are `@uncli/chat` (`app/frontend/packages/chat`); a Svelte host can use the components directly. `@uncli/client` (`app/frontend/packages/client`) is the library underneath. Neither is published yet. |

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
- **`compact`** shows the phone's compact layout, and `compact="false"` the desktop's. Leave it out and the element picks by its own width: compact below 720 px.
- **`storage-key`** remembers the side panel and the compact layout under `uncli-layout:<key>` in the page's local storage. Without it the chat remembers nothing, so UNCLI never writes to your storage unasked. The side panel starts closed.

The element fills the box you give it, so give it a height. In a grid or flex layout, let its container shrink (`min-width: 0`).

## A list of sessions

`<uncli-session-list>` shows a server's sessions as UNCLI's sidebar does:
- drag or Alt+arrows to reorder;
- rename, archive and delete;
- the archived sessions apart.

Picking a session doesn't open it. The list sends `uncli-select` and your page decides what to show:

```js
list.backend = chat.backend // the same backend: one client for both
list.addEventListener('uncli-select', e => {
  chat.session = e.detail.session
  list.selected = e.detail.session // marked as the one shown
})
```

Its attributes are `selected` and `scheme`.

## Events

The element sends these events. They bubble and are composed, so you can listen on the element or the document. `detail` is `{ session }`.

| Event | When |
|---|---|
| `uncli-ready` | The chat has the server's state and shows its session (a list: has its sessions) |
| `uncli-select` | The user picked a session in a list |
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

## Your links, your strings, your keys

The `host` property is what your page does for the chat:

```js
chat.setAttribute('link-schemes', 'bruv') // keep bruv:… links in answers
chat.host = {
  // Every link clicked in an answer comes here first: true when you took it.
  openLink: href => href.startsWith('bruv:') && (openCard(href), true),
  // Your text for one of UNCLI's strings (keys as in its en.json), or
  // nothing to keep UNCLI's.
  t: (key, params) => myStrings[key],
}
```

- **Link schemes.** Links in answers keep only `http(s)`, `mailto`, `tel` and `file` targets unless `link-schemes` names your own. Schemes that can run code or carry content (`javascript`, `data` and the like) are never kept.
- **Links you don't take.** They open as in UNCLI: web links in the browser, and file links at the server's screen.
- **One host per page.** The schemes and strings apply to every chat on the page.

**Keys.** Before the chat acts on Escape, or on Ctrl+Enter (Cmd+Enter), it sends `uncli-escape` or `uncli-submit`. They're cancelable: cancel one (`event.preventDefault()`) and the chat leaves that key alone, so layered dialogs of your own can decide first.

## What's not there yet

- **`complete` for @mentions and `turnContext`.** They need the host protocol below.
- **Named slots** (`header-extra`, `below-composer`) and **`::part()` names.**
- **React, Vue and Angular wrappers,** generated from a Custom Elements Manifest.
- **Host credentials and host sessions** (the host protocol). Until then, a host reaches UNCLI as a paired device.

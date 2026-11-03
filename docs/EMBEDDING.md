# Embedding UNCLI

UNCLI's chat can be shown in any web page as the `<uncli-chat>` element
(decision 0026). It's the same Svelte chat UNCLI's own app shows, shipped as
a standard custom element, so it works in any stack. This page is for host
authors. It covers what exists today and says what doesn't yet.

## Status and getting it

| | |
|---|---|
| **Status** | Alpha, on the `feat/packages` branch. Not published to a registry yet. Your page can't reach a real UNCLI yet: build against the mock (see "What's not there yet"). |
| **Packages** | `@uncli/chat` (`app/frontend/packages/chat`): the elements, built, as `@uncli/chat/element`, and the Svelte components as source. `@uncli/client` (`app/frontend/packages/client`): the client underneath, and its mock as `@uncli/client/mock`. |
| **Getting them** | `npm --prefix app/frontend run pack:packages` builds the element and packs both packages as tarballs into `app/frontend/dist-embed/packs/`. A host installs the two tarballs (`npm install <path>/uncli-client-….tgz <path>/uncli-chat-….tgz`). |
| **A sample** | `npm --prefix app/frontend run build:embed` also builds a sample host page on UNCLI's mock, into `dist-embed/sample/`. |

Import the element once, anywhere in your app:

```js
import '@uncli/chat/element' // registers <uncli-chat> and <uncli-session-list>
```

It's built already, Svelte inside, so your bundler needs nothing special and your stack can be anything. A Svelte host can also mount the components themselves (`@uncli/chat/components/Chat.svelte` on a `ChatStore`), but then it styles and provides them like UNCLI's own app does. The element is the supported way in.

## Showing a chat

```html
<uncli-chat session="s-code" scheme="dark" storage-key="card-chat"></uncli-chat>
<script type="module">
  import '@uncli/chat/element' // registers <uncli-chat>
  import { httpBackend } from '@uncli/client'

  const chat = document.querySelector('uncli-chat')
  chat.backend = httpBackend({ base: 'https://pc.tailnet.ts.net:9871', token: () => deviceToken })
</script>
```

- **`backend`** (property) is how the chat reaches UNCLI.
  - `httpBackend` reaches a UNCLI over its remote access server (decision 0021), with a paired device's token. **Not from your page yet:** that server refuses requests from any page it didn't serve itself (decision 0022), so today `httpBackend` works only in UNCLI's own phone app. Host access is being designed (see "What's not there yet").
  - Any object implementing `Backend` works, including `@uncli/client/mock`, which is what to build against for now.
  - Every element given the same backend object shares one client: one event stream, one copy of the server's state.
- **`session`** (attribute or property) is the session to show. Changing it shows another.
- **`scheme`** is `light` or `dark`. Leave it out to follow the system.
- **`compact`** shows the phone's compact layout, and `compact="false"` the desktop's. Leave it out and the element picks by its own width: compact below 720 px.
- **`storage-key`** remembers the side panel and the compact layout under `uncli-layout:<key>` in the page's local storage. That's the only thing UNCLI ever writes to your storage, and only when you ask. Everything else the chat remembers (drafts, scroll positions, the File tab, the approval scope) stays in memory for as long as the page is open. The side panel starts closed.

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

- **Measured size** (2026-11-03): about 218 KB gzipped as built, and about 184 KB once your bundler minifies it.
- **Load it lazily:** a dynamic `import()` when a chat first opens.

## Your links, strings, mentions and context, and your keys

The `host` property is what your page does for the chat:

```js
chat.setAttribute('link-schemes', 'bruv') // keep bruv:… links in answers
chat.host = {
  // Every link clicked in an answer comes here first: true when you took it.
  openLink: href => href.startsWith('bruv:') && (openCard(href), true),
  // Your text for one of UNCLI's strings (keys as in its en.json), or
  // nothing to keep UNCLI's.
  t: (key, params) => myStrings[key],
  // @mentions: what matches the word typed after an @. Picking one puts
  // its insert in the message.
  complete: async query => (await searchCards(query)).map(c => ({ label: c.title, detail: 'card', insert: `[${c.title}](bruv:card:${c.id})` })),
  // What the user has in front of them now, asked for as each turn is sent.
  turnContext: () => describeOpenCard(),
}
```

- **Link schemes.** Links in answers keep only `http(s)`, `mailto`, `tel` and `file` targets unless `link-schemes` names your own. Schemes that can run code or carry content (`javascript`, `data` and the like) are never kept.
- **Links you don't take.** They open as in UNCLI: web links in the browser, and file links at the server's screen.
- **Mentions.** `complete` may answer at once or with a promise. While the user types an `@` and a word, the menu shows up to eight matches; Up, Down, Tab or Enter, and Escape work as in the slash-command menu.
- **Turn context.** `turnContext` is told to the model with that turn only, among the instructions the chat hides, quoted as what your app says the user has in front of them. The model weighs it as context, not as the user's request. Up to 8,000 characters are kept. It needs a UNCLI that has the `SendWith` call (added on the `feat/packages` branch); a turn without context is sent as before and works with any UNCLI.
- **One host per page.** The schemes and strings apply to every chat on the page.

**Keys.** Before the chat acts on Escape, or on Ctrl+Enter (Cmd+Enter), it sends `uncli-escape` or `uncli-submit`. They're cancelable: cancel one (`event.preventDefault()`) and the chat leaves that key alone, so layered dialogs of your own can decide first.

## Your own controls, and restyling parts

**Slots.** Put your own controls inside the chat by giving the element children with a `slot` attribute. They stay in your page's DOM and get your page's styles.

| Slot | Where |
|---|---|
| `header-extra` | Under the chat's header (a pending-edits banner, say) |
| `below-composer` | Under the message box |

```html
<uncli-chat session="s-code">
  <button slot="header-extra">3 edits waiting for review</button>
</uncli-chat>
```

**Parts.** Beyond the `--uncli-*` tokens, these parts can be restyled with `::part()`, for example `uncli-chat::part(composer) { border-top: 2px solid var(--brand); }`. Both layouts have the same parts.

| Element | Parts |
|---|---|
| `<uncli-chat>` | `header`, `page`, `approvals`, `navbar`, `composer`, `side-panel` |
| `<uncli-session-list>` | `list`, `item`, plus `item-current` on the one shown |

The part names are part of the contract; the structure inside them isn't.

## What's not there yet

- **React, Vue and Angular wrappers,** generated from a Custom Elements Manifest.
- **Reaching a real UNCLI from your page.** A host needs its own credentials, approved in UNCLI, and UNCLI's server has to accept requests from the host's origin. Until then, the chat runs only on the mock in your page.
- **UNCLI inside your app,** without a separate install: a headless UNCLI your app starts itself, or links in if it's written in Go.

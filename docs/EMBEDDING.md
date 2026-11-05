# Embedding UNCLI

Your app can run a UNCLI of its own and show its chat in your pages, so
you don't have to build an AI chat harness yourself (decisions 0026 and
0027). It comes in two parts:

- **The core**, UNCLI's Go side, which runs the CLIs. Your app runs it
  privately: in your Go program, or as the `uncli-core` program beside any
  other server. Nothing is shared with other apps or an installed UNCLI.
- **The elements**: `<uncli-chat>` and `<uncli-session-list>`, the same
  Svelte chat UNCLI's own app shows, shipped as standard custom elements,
  so they work in any stack.

Your server forwards a path, `/uncli/` say, to the core, and your pages
show `<uncli-chat src="/uncli">`. This page is for host authors. It covers
what exists today and says what doesn't yet.

## Status and getting it

| | |
|---|---|
| **Status** | Alpha, on the `feat/embed` branch. Not published to a registry yet. Windows only, like UNCLI. |
| **The core, for Go** | `github.com/harvey-withington/uncli/app/uncli` (the `app/uncli` folder). |
| **The core, for anything else** | `uncli-core.exe`: `go build ./cmd/uncli-core` in `app/`. Releases will carry it too, as `uncli-core-<version>-windows-x64.exe` with its checksum in `SHA256SUMS.txt`. |
| **Packages** | `@uncli/chat` (`app/frontend/packages/chat`): the elements, built, as `@uncli/chat/element`, with TypeScript types (`element.d.ts`), and the Svelte components as source. `@uncli/client` (`app/frontend/packages/client`): the client underneath, and its mock as `@uncli/client/mock`. |
| **Go dependencies** | Importing `app/uncli` needs Go 1.26, and brings UNCLI's own dependencies in as indirect ones, Tailscale's client among them, with newer `golang.org/x/*` than you may have. |
| **Getting them** | `npm --prefix app/frontend run pack:packages` builds the element and packs both packages as tarballs into `app/frontend/dist-embed/packs/`. A host installs the two tarballs (`npm install <path>/uncli-client-….tgz <path>/uncli-chat-….tgz`). |
| **Samples** | `npm --prefix app/frontend run build:embed` builds a sample host page into `dist-embed/sample/`. On its own it runs on UNCLI's mock. Served by a host, opened with `?src=/uncli`, it uses that host's UNCLI: `app/cmd/uncli-sample-host` is one in Go, `app/frontend/embed-sample/node-host.mjs` one in Node. |

## Running your own UNCLI

**In a Go program:**

```go
import "github.com/harvey-withington/uncli/app/uncli"

func main() {
	uncli.RunHook() // first thing, with Hooks: true (see below)
	core, err := uncli.Start(uncli.Options{
		DataDir: dataDir,                         // everything it keeps
		Origins: []string{"https://app.example"}, // your pages' origin
		Name:    "My app",                        // recorded on approvals your pages answer
		Hooks:   true,
	})
	if err != nil { log.Fatal(err) }
	defer core.Close()
	mux.Handle("/uncli/", http.StripPrefix("/uncli", requireSignIn(core.Handler())))
	…
}
```

**With any other server:** start `uncli-core` with a key you make, read the port it prints, and forward `/uncli/` to it, adding the key:

```
uncli-core --data <folder> --origin https://app.example --name "My app"   (the key in UNCLI_CORE_KEY, or --key-file)
→ {"port":51234}
```

- Give it a pipe on its standard input. It stops when that closes, so it ends with your server however your server ends.
- It listens on loopback only, and accepts only requests carrying the key.
- Your proxy must not buffer `/uncli/events`, a long-lived event stream. Node's `http` and Go's `httputil.ReverseProxy` pass it through as it comes; nginx-style proxies need buffering off.
- `node-host.mjs` is a whole host in about sixty lines, with no packages.

**Who can use it is your business.** Anyone you let reach `/uncli/` can make the CLIs run tools as your server's user, on its machine. Put your own sign-in in front of it. Approvals, prompt levels and the safe list apply as they do in UNCLI.

**Run as a person, not a service account.** The CLIs run tools as the account your server runs as. UNCLI refuses to start as one of Windows' service accounts (LocalSystem, LocalService, NetworkService): LocalSystem can do anything on the machine.
- If your server is a Windows service, set its *Log on as* to a person's account.
- Check your installer and updates keep that account. One that deletes and recreates the service puts it back on LocalSystem.
- `Start` then returns an error that `errors.Is(err, uncli.ErrServiceAccount)` finds; `errors.As` gives a `*uncli.ServiceAccountError` with the account's name, for your own message.
- `AllowServiceAccount` (`--allow-service-account`) overrides the refusal, if you really mean it.

**Your sign-in, cookie or token.** UNCLI sets no cookies and needs none.
- **Cookie sign-in:** the browser sends your cookie with the chat's requests by itself.
- **Token in a header:** give the chat a `host.headers` function (below), such as `() => ({ Authorization: 'Bearer ' + token })`. It's asked for on every request, so a renewed token is used at once. In Go, `Handler()` swaps in UNCLI's key after your middleware has checked yours. A proxy does the same.
- **Signed links:** links to artifacts and uploads open in a new tab, which can't send headers. They're signed by UNCLI, expire, and work only on their own route, so let `/uncli/l/` through your sign-in.

**Pages from another origin.** Your page may come from somewhere other than your server, such as a desktop app's webview (Wails serves from `http://wails.localhost:<port>`). Then:
- give `src` your server's full address, e.g. `src="https://app.example:9870/uncli"`;
- list the page's origin in `Origins`.

UNCLI answers the browser's preflight for those origins and lets them read its replies. It never allows credentials (cookies), so send your sign-in with `host.headers`. If your sign-in sits in front of `/uncli/`, let `OPTIONS` requests through to UNCLI, or answer CORS yourself. If you answer it yourself, allow the `Authorization`, `Content-Type` and `Last-Event-ID` headers: the event stream resumes with `Last-Event-ID`, so without it a reconnect fails its preflight.

**Other sites can't drive it.** A request a page sends from an origin not in `Origins` (`--origin`) is refused before it runs, so another site can't use your users' cookies for your app to reach it. Requests without an `Origin`, such as your server's own, pass. A server reached by several names (an address, a tailnet name, dev ports) can decide in Go with `AllowOrigin: func(origin string, r *http.Request) bool`, such as "the origin's host is the request's own, or a loopback dev port". Answer no to anything you don't recognise.

**What stays where.** Everything the core keeps lives in its data folder:
- its database and settings;
- the CLIs it downloads, and their session folders;
- the CLIs' own config: sign-in, transcripts, memory, skills, MCP servers.

Your users sign in to the CLIs once for your app, and your app doesn't see their personal CLI setup. Only the CLIs your UNCLI starts are told where that folder is: your process's environment is left alone, so a terminal, script or tool your app starts itself still uses the user's own CLI setup. `UserCLIConfig` (`--user-cli-config`) uses the user's own CLI config instead, shared with everything else on the machine that uses those CLIs.

**The data folder is your app's alone.**
- One UNCLI at a time: a second one started on the same folder is refused while the first runs, the desktop app included.
- The first start marks the folder with your app's `Name`. Another name is refused, so two apps can't share a folder by mistake. If your app is renamed, delete `uncli-owner.json` in the folder.
- UNCLI's own folders (`%AppData%\uncli`, `%LocalAppData%\uncli`, and anything inside them) are always refused.

These checks catch mistakes, not malice. Your UNCLI is as trusted as your app: a program running as your user can read any folder of that user's without UNCLI. So keep `--key-file` somewhere only your user can read, or pass the key in `UNCLI_CORE_KEY`.

**Setting up the CLI.** Until your UNCLI has its CLI downloaded and signed in, `<uncli-chat>` shows the steps in its place:
1. Download: UNCLI's pinned copy, checked against its published checksum, into your data folder.
2. Sign in: your user opens the sign-in page with a button (your `host.openLink` sees the link first, so a desktop app can open it in the system browser rather than its webview), signs in, and pastes back the code it shows.

Once both are done, the chat appears. Your page needs no code for this.

**What the core leaves to you:**
- notifications (listen for `uncli-needs-approval`);
- updates: the core is the version you ship.

Containers aren't available in a host's UNCLI yet.

**Hooks.** Some CLIs ask UNCLI about each tool call through a hook: a program they run (Antigravity, decision 0012). With `Hooks: true`, that program is yours, so call `uncli.RunHook()` first thing in `main`: it answers the hook and exits, and otherwise returns at once. Without it, those CLIs' sessions don't start. `uncli-core` does this itself.

Import the element once, anywhere in your app:

```js
import '@uncli/chat/element' // registers <uncli-chat> and <uncli-session-list>
```

It's built already, Svelte inside, so your bundler needs nothing special and your stack can be anything. A Svelte host can also mount the components themselves (`@uncli/chat/components/Chat.svelte` on a `ChatStore`), but then it styles and provides them like UNCLI's own app does. The element is the supported way in.

## Showing a chat

```html
<uncli-chat src="/uncli" session="…" scheme="dark" storage-key="card-chat"></uncli-chat>
<script type="module">
  import '@uncli/chat/element' // registers <uncli-chat>
</script>
```

- **`src`** is the path your server forwards to your UNCLI. Every element with the same `src` shares one client: one event stream, one copy of the core's state. The page sends no key; your server adds it.
- **`backend`** (property) instead takes any object implementing `Backend`, such as `@uncli/client/mock` to build against without a core.
- **`session`** (attribute or property) is the session to show. Changing it shows another.
- **`newSession(profile?, workdir?, model?, { meta? })`** starts a session and shows it, returning its id. The profiles are `chat` (the default: a fresh folder each time), and `cowork` and `code`, which work in the `workdir` you give. `meta` is your own word on what the session is for, such as `card:42`. Your server reads it back with `core.SessionMeta(id)`, from the session's very first start, before you've recorded anything yourself; that's what `MCPServers` needs. It's kept while your UNCLI runs.
- **When it connects.** A moment after it gets `src` or `backend`, so a `host` you set in the same go (with your sign-in headers) is in place for the first request.
- **The client** the elements use is exported from `@uncli/chat/element` too (`chatClient`, `mountedBackend`). Import it from there, not from `@uncli/client`, so your code shares the elements' connection rather than opening its own.
- **`scheme`** is `light` or `dark`. Leave it out to follow the system.
- **`compact`** shows the phone's compact layout, and `compact="false"` the desktop's. Leave it out and the element picks by its own width: compact below 720 px.
- **`storage-key`** remembers the user's layout and preferences under `uncli-layout:<key>` in the page's local storage. That's the only thing UNCLI ever writes to your storage, and only when you ask. It remembers:
  - the side panel: open or closed, its width, and its tab (On this page, Artifacts or File);
  - the desktop header, collapsed or not;
  - the question header, collapsed or not;
  - the compact header's Options, open or closed;
  - the scope "This is safe" last applied to (this project, or all of them).

  Every chat on the page with the same key shares this memory. A change in one shows in the others at once, and a chat you mount later starts from it. So give all your chats one key, unless you want them to differ. It's read each time a chat mounts, so it survives your page remounting the element, and the switch between the compact and desktop layouts.

  Without a key, a chat remembers these only while it's mounted.

  What belongs to one chat isn't remembered here: drafts, scroll positions and the File tab's file. Those stay in memory for as long as the page is open. MCP servers the user turns off aren't remembered here either: your UNCLI saves that choice itself, for every chat (see "Your own tools for the AI"). The side panel starts closed.

The element fills the box you give it, so give it a height. In a grid or flex layout, let its container shrink (`min-width: 0`).

## A list of sessions

`<uncli-session-list>` shows a server's sessions as UNCLI's sidebar does:
- drag or Alt+arrows to reorder;
- rename, archive and delete;
- the archived sessions apart.

Picking a session doesn't open it. The list sends `uncli-select` and your page decides what to show:

```js
list.setAttribute('src', '/uncli') // the chat's src: one client for both
list.addEventListener('uncli-select', e => {
  chat.session = e.detail.session
  list.selected = e.detail.session // marked as the one shown
})
```

Its attributes are `src`, `selected` and `scheme`, and it takes a `backend` property as the chat does.

## Events

The element sends these events. They bubble and are composed, so you can listen on the element or the document. `detail` is `{ session }`.

| Event | When |
|---|---|
| `uncli-ready` | The chat has the server's state and shows its session (a list: has its sessions) |
| `uncli-select` | The user picked a session in a list |
| `uncli-busy` | The session started answering |
| `uncli-idle` | It finished |
| `uncli-needs-approval` | A tool call, or a review you raised, is waiting for the user |
| `uncli-missing` | The session you gave isn't on this UNCLI (deleted, say): start a new one |
| `uncli-deleted` | The user deleted the chat from its header |
| `uncli-new-chat` | The user chose New chat in its header. Cancelable: cancel it and call `newSession` yourself, with your `meta`; otherwise the chat starts a new session of the same type itself |
| `uncli-new-session` | It started that new session and shows it; `detail` is `{ session, previous }` |

In a host's page, the chat's header has **New chat** and **Delete chat**, since your page may show no session list. Delete asks first.

With TypeScript, `@uncli/chat/element`'s types name each event in `HTMLElementEventMap`, so `addEventListener('uncli-new-session', e => e.detail.previous)` is typed.

## Looking like your app

The chat lives in a shadow root:
- Your page's styles don't reach it, and its styles don't reach your page.
- Two names can still be the same (`--bg`, `.btn`) without clashing.

You theme it through UNCLI's public tokens (decision 0010):
- Set any `--uncli-*` custom property on the element, usually as a reference to your own tokens: `uncli-chat { --uncli-accent: var(--brand); }`. Your tokens may share UNCLI's working names (`--uncli-accent: var(--accent)` is fine).
- When you set `--uncli-accent`, set `--uncli-accent-text` too: the colour of text and icons on it, such as the Send arrow. UNCLI's own is near-black on its light accent, which is unreadable on a dark one.
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

## Your own tools for the AI

Give your sessions your app's own tools, so the AI can work on your things, such as cards on a board. Serve them as an MCP server over HTTP (your server's own route is fine) and name them in `MCPServers`:

```go
core, err := uncli.Start(uncli.Options{
	…
	MCPServers: func(session string) map[string]uncli.MCPServer {
		board := boardOf(session) // which of your things this session is about
		return map[string]uncli.MCPServer{
			"board": {URL: "http://127.0.0.1:9870/repos/" + board + "/mcp", Headers: map[string]string{"Authorization": "Bearer " + tokenFor(board)}},
		}
	},
})
```

- **When it's asked.** Each time a session's CLI starts, with the session's id (what `newSession()` returned). So each session can have the servers for what it's about, with a token that's current then.
- **Every session type.** It works for chats, cowork and code sessions alike, beside whatever servers the session type shares of the user's own.
- **What the session is for.** For a session made with `newSession(…, { meta })`, `core.SessionMeta(session)` tells you, from its very first start.
- **Servers the user turns off.** The CLI may load MCP servers that aren't yours, such as the claude.ai connectors of the account it's signed in with. One that needs signing in shows the attention dot. The user can choose **Turn off** for it in the MCP servers list. Your UNCLI saves that in its data folder, for every chat, and turns the server off again each time a CLI starts. It shows as "off", with **Turn on** to bring it back.
- **Approvals.** By default, calls to your tools are approved like any MCP server's. A tool marked read-only in its MCP annotations (`readOnlyHint`) runs without asking under the default prompt level; others ask in the chat, where your user can allow them once or always.
- **Not yet in `uncli-core`.** For now, `MCPServers` is only available to Go hosts.

**Your own approvals for your own tools.** `HostToolPolicy` lets your app decide instead, such as a Chat / Suggest / Edit mode of its own. It's asked at each call, with the call's arguments, so a mode switch takes effect at once and you can say what this particular call does:

```go
HostToolPolicy: func(call uncli.HostToolCall) uncli.HostDecision {
	// call.Session, call.Server, call.Tool, and call.Args (the arguments, as a map)
	if !writes(call.Tool) {
		return uncli.HostDecision{} // UNCLI's own rules
	}
	switch modeOf(call.Session) {
	case "chat":
		return uncli.HostDecision{Decision: uncli.PolicyDeny, Reason: "Chat mode: the board can't be changed."}
	case "suggest":
		return uncli.HostDecision{Decision: uncli.PolicyAsk, Reason: "Suggest mode", Prompt: &uncli.HostPrompt{
			Title:  fmt.Sprintf("Update checklist '%v'", call.Args["checklist"]),
			Detail: describeChanges(call.Args), // "Adds: Passport\nTicks: Book taxi"
		}}
	}
	return uncli.HostDecision{Decision: uncli.PolicyAllow, Reason: "Edit mode"}
},
```

- **`PolicyAllow`** runs the tool without asking, even at the strictest prompt level.
- **`PolicyDeny`** refuses it, and the model is told your `Reason`.
- **`PolicyAsk`** asks the user on UNCLI's approval card, even where UNCLI wouldn't. With a `Prompt`, the card shows your `Title` in place of the tool's name and arguments, and your `Detail` under it (plain text, line breaks kept). Without one, it shows the tool as UNCLI would.
- **`""`** leaves it to UNCLI's rules.

**Your prompt on the card.** It's an approval like any other:
- It shows in both layouts.
- It sends `uncli-needs-approval`, and counts towards the folded header's attention dot.
- Allow once runs the tool, and Deny refuses it.
- "This is safe" remembers the tool, in UNCLI's scope (this project or all of them). From then on the tool runs without you being asked, since the user chose "always". `PolicyDeny` still wins.

**One review for a batch of changes.** Asking on each call means a turn that makes twenty changes asks twenty times. To ask once instead, let the calls through and keep the changes aside yourself (your tool answers `{"status":"staged"}` so the model knows nothing has changed yet). When the turn ends (`uncli-idle`), raise one card with the element's `review()`:

```js
chat.addEventListener('uncli-idle', async () => {
  const staged = await myStagedChanges(chat.session)
  if (!staged.length) return
  const { action, items } = await chat.review({
    title: `Apply ${staged.length} suggested changes?`,
    detail: 'From this turn.',
    items: staged.map(c => ({ id: c.id, label: c.summary, detail: c.more })),
  })
  if (action === 'apply') await applyStaged(items) // the ids still ticked
  else if (action === 'discard') await discardStaged()
})
```

The card sits where approval cards do, in UNCLI's look, in both layouts. Each item starts ticked; the user can untick some, or use All and None. Apply waits for at least one tick. Pass `actions` (`{ id, label, tone }`, tone `primary`, `danger` or `neutral`) for your own buttons in place of Apply and Discard; leave out `items` for an all-or-nothing card. The promise resolves with the button's `id` and the ticked items' ids (all of them for a card without items). The card sends `uncli-needs-approval` and counts towards the folded header's attention dot. It belongs to the session shown when you raised it (or `{ session }` in the second argument), and stays until it's answered, even if the user moves to another chat and back. Pass `{ signal }` (an `AbortSignal`) to withdraw it: the promise then resolves with `action: 'dismissed'`.

Only your own servers' tools are asked about: those `MCPServers` gave the session. The CLI's own tools and the user's own MCP servers keep UNCLI's rules, so your app can't loosen their safety. The chat's trace shows "allowed by the app", with your reason.

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
  // Your own sign-in, sent with every request to src, when it's a token
  // rather than a cookie.
  headers: () => ({ Authorization: `Bearer ${myToken()}` }),
  // What Enter does in the message box, to match your own chat: "send"
  // (Shift+Enter for a new line) or "newline" (Ctrl+Enter sends). Left out:
  // the viewer's choice, or a new line on touch screens.
  enter: "newline",
  // Your own controls in the message box, drawn by UNCLI (below).
  composerControls: [/* … */],
}
```

**Your controls in the message box.** `composerControls` puts controls of your own in the message box's right-hand column, above Attach and Send, so the text keeps its full height. They're drawn in UNCLI's style so they fit it on a phone and the desktop alike:

```js
chat.host = {
  …,
  composerControls: [
    // Pick one of several: a drop-down showing the current one.
    { id: 'mode', label: 'Mode', value: mode, onChange: setMode, options: [
      { id: 'chat',    label: 'Chat',    icon: 'message-circle', tone: 'success', hint: "Can't change the board" },
      { id: 'suggest', label: 'Suggest', icon: 'list-checks',    tone: 'warning', hint: 'Changes wait for your review' },
      { id: 'edit',    label: 'Edit',    icon: 'pencil-line',    tone: 'danger',  hint: 'Changes apply at once' },
    ] },
    // A toggle: each click moves to the next state (Shift+click back); two states are on/off.
    { kind: 'toggle', id: 'length', label: 'Answers', value: length, onChange: setLength, states: [
      { id: 'brief', label: 'Brief', icon: 'shrink' },
      { id: 'full',  label: 'In full', icon: 'list-checks', tone: 'accent' },
    ] },
    // A single click: an action (or an options list of one).
    { id: 'review', label: 'Review edits', icon: 'list-checks', onClick: openReview },
  ],
}
```

- **Each item:** a `label`, an `icon` (a Lucide name from UNCLI's set; unknown names show a neutral icon), a `tone` (`accent`, `success`, `warning`, `danger` or `muted`, from the colours you theme) and a `hint`, shown as the tooltip.
- **Kind:** several `options` make a drop-down and one makes a plain button; `kind: 'toggle'` makes a toggle.
- **Values are yours.** `onChange` tells you what was picked. Set `host` again with the new `value` to show it, or don't, to refuse.
- **Narrow boxes:** in a message box below about 300 px, buttons and drop-downs show their icons only, with the label still as their name and tooltip.
- **Anything else** goes in the `composer-controls` slot, in the same place, with your own markup and styles.

- **Link schemes.** Links in answers keep only `http(s)`, `mailto`, `tel` and `file` targets unless `link-schemes` names your own. Schemes that can run code or carry content (`javascript`, `data` and the like) are never kept.
- **Links you don't take.** They open as in UNCLI: web links in the browser, and file links at the server's screen.
- **Mentions.** `complete` may answer at once or with a promise. While the user types an `@` and a word, the menu shows up to eight matches; Up, Down, Tab or Enter, and Escape work as in the slash-command menu.
- **Turn context.** `turnContext` is told to the model with that turn only, among the instructions the chat hides, quoted as what your app says the user has in front of them. The model weighs it as context, not as the user's request. Up to 8,000 characters are kept.
- **One host per page.** The schemes, strings and headers apply to every chat on the page. A page with only a `<uncli-session-list>` sets its headers with `setHostHeaders(fn)`, exported from `@uncli/chat/element`.

**Keys.** Before the chat acts on Escape, or on Ctrl+Enter (Cmd+Enter), it sends `uncli-escape` or `uncli-submit`. They're cancelable: cancel one (`event.preventDefault()`) and the chat leaves that key alone, so layered dialogs of your own can decide first.

Your own single-key shortcuts need care, because the chat lives in a shadow root. While the user types in the message box, `document.activeElement` is the `<uncli-chat>` element itself, not the text box. A check like "is an input focused?" then says no, and your shortcut takes the keystroke. Look inside the element too:

```js
function typing() {
  let el = document.activeElement
  while (el?.shadowRoot?.activeElement) el = el.shadowRoot.activeElement
  return el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement || el?.isContentEditable
}
```

Or, inside a key handler, check where the key came from: `event.composedPath()[0]` is the text box.

## Your own controls, and restyling parts

**Slots.** Put your own controls inside the chat by giving the element children with a `slot` attribute. They stay in your page's DOM and get your page's styles.

| Slot | Where |
|---|---|
| `header-start` | At the start of the header's top line |
| `header-end` | At the end of the header's top line (your close button, say), so your header and the chat's can be one |
| `header-extra` | Under the chat's header (a pending-edits banner, say) |
| `composer-controls` | Inside the message box, above Attach and Send, after any `composerControls` |
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

- **Setting up the other providers' CLIs** in your page: the chat sets up the first one only.
- **Remote access for your own phone app,** switched on for your UNCLI.
- **Containers** in a host's UNCLI.
- **React, Vue and Angular wrappers,** generated from a Custom Elements Manifest.

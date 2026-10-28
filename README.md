# UNCLI

AI CLIs if you're not a CLI guy.

UNCLI is a desktop app for running AI coding CLIs without a terminal:
Claude Code and the Antigravity CLI built in, and more through provider
plugins. Run several sessions side by side (Chat, Co-work on a folder, or
Code on a repository), read each answer as a page you can bookmark and copy
from, approve what the agent does on cards that say what it means, and
switch models or modifiers like Efficient or Thorough with a click. It drives each
provider's official CLI with your own subscription; sessions never call a
model API directly.

Website: <https://uncli.app>

## Install

Requires Windows 10 or 11 (x64). Download the installer from
[GitHub Releases](https://github.com/harvey-withington/uncli/releases/latest);
`SHA256SUMS.txt` beside it has its checksum. The installer adds Microsoft's
WebView2 runtime if Windows doesn't have it yet.

UNCLI installs for you alone, in `%LocalAppData%\Programs\UNCLI`, so
installing and updating never ask for an administrator (decision 0024). If
an older version is installed for everyone in Program Files, the installer
offers to remove it, and Windows asks for permission once.

UNCLI looks for a newer release by itself and offers to install it: Update
now downloads it, checks its checksum, then quits, installs and starts UNCLI
again (decision 0025). Settings → Updates checks on demand, or turns the
automatic checks off. Builds you make yourself (`npm run build`) don't
check; test builds from `npm run publish-build` carry the last release's
version plus `+dev.<build>`, so they're offered the next release.

On first run UNCLI downloads its own copy of Claude Code (the version this
release is tested with, about 240 MB, checked against Anthropic's published
checksum) and asks you to sign in with your Claude account in the browser.
Other providers are installed the same way from Settings → AI Providers.
UNCLI doesn't use or change any copy of a CLI you already have installed.

App data (sessions, pages, bookmarks) lives in your user config folder
(`%AppData%\uncli` on Windows); the downloaded CLIs in your user cache
folder (`%LocalAppData%\uncli\cli`). Chat sessions get their own folder under
`%AppData%\uncli\scratch`. Uninstalling UNCLI leaves both folders; delete
them to remove everything.

### What goes over the network

UNCLI has no account, telemetry or update check of its own. It connects to:

- each provider's download host, to install the pinned CLI you chose;
- the Alpine Linux mirror, when you build a container;
- a decision model's endpoint, only if you set one up in Settings;
- with Remote access on, the Tailscale app on this PC (for its address and
  HTTPS certificate) and, for paired devices that turned notifications on,
  their browsers' push services (end-to-end encrypted).

Everything else is the provider's CLI talking to its own service, under
your account with that provider.

To restyle UNCLI, put a `theme.yaml` in that folder: colours, fonts and
radii for light and dark, by the token names in the "Theme override" section
of `docs/UI-CONVENTIONS.md`. Run **Reload theme file** from the palette to
apply changes.

## Use

- **New session** (Ctrl+N): pick Chat, Co-work (a folder) or Code (a
  repository), the AI provider and a model.
- **Chat** reads none of your CLI settings or instructions. Each chat can
  still use your MCP servers (the ones added for the CLI everywhere,
  `claude mcp add -s user`) and claude.ai connectors, and your skills: tick
  them in the new-session dialog, or change them later from the note after
  the chat's folder in its header (decision 0016). MCP tools load only when
  a turn needs them, and each tool use is approved like any other. The
  chat profile's `mcp`, `connectors` and `skills` in your `profiles.yaml`
  set what a new chat starts with.
- **MCP sign-in**: a server that needs signing in says so under the
  session's folder. **Sign in** opens its page in your browser, and its
  tools arrive without a restart, in your other sessions too. The plug
  button in the header lists every server, to reconnect or sign out
  (decision 0020).
- **AI providers**: Claude Code, and the Antigravity CLI (Google). Settings
  → AI Providers installs each one's pinned CLI and says whether it's
  signed in. The Antigravity CLI uses the Google sign-in of the Antigravity
  app on the same computer. Its tool calls are approved on the same cards
  through a hook UNCLI gives it (decision 0012); it can't be sent files,
  and stopping an answer stops its process.
- **Provider plugins** add more CLIs without code: a `provider.yaml` in the
  config folder's `providers/<id>/` describes the CLI, and UNCLI runs it
  (decision 0013, [docs/PLUGINS.md](docs/PLUGINS.md)). A plugin does nothing
  until you enable it in Settings → AI Providers, which shows what it would
  download and run first. CLIs that speak ACP need the least
  (decision 0014): Grok Build is one (`app/testdata/providers/grok`), signed
  in with a code you approve on any device.
- Each question and its answer is a **page**. Page with ← and →, bookmark
  with B, and jump between bookmarks with the double arrows. Hover any
  paragraph, code block or the question to copy it (paragraphs copy their
  original markdown).
- The toolbar switches model mid-conversation, toggles modifiers (Use
  Agents, Efficient / Normal / Thorough), runs `/compact` and `/context`, and
  shows your subscription usage.
- **Prompt me** (Always, When unsafe or Never) decides when the agent stops
  to ask before using a tool. A card shows what it wants to do, and "This is
  safe" teaches UNCLI for next time. The shield opens the safe list.
- Each page lists the **files the agent changed**. In Code sessions, Open takes
  you to the line in your editor (Settings → Editor).
- Chat and Co-work sessions keep their **artifacts** (HTML, SVG, Mermaid,
  Markdown, images) version by version. The side panel's Artifacts tab (or
  A) shows them as they were at the page you're on, beside On this page (O).
- **Watch a file** from the session folder on the side panel's File tab (F):
  a TODO list, a plan or a log, rendered and updated as it changes. Type part
  of a name to find one, latest changed first; star the ones you come back to.
- A **desktop notification** says when a session you aren't looking at
  finishes or needs approval (Settings → Notifications).
- **Import** a conversation Claude Code saved (New session → Continue a
  conversation from the terminal…) to read it in UNCLI and carry on. A
  deleted UNCLI chat can be brought back the same way.
- **Pin** pages you want at hand (P), **archive** finished sessions, see
  **usage** over time, and find any action in the **command palette**
  (Ctrl+Shift+P); ? lists every shortcut.
- **Containers** (Windows): run a session in a Linux that UNCLI builds with
  WSL, where the agent sees only the session's folder and can't start Windows
  programs. Settings → Containers turns WSL on (one administrator prompt and
  a restart), builds containers and signs them in. Then pick the container
  under Run in when you start a session. The built-in Sandbox shares your
  memories, skills, MCP servers and, once you sign in with your whole
  account (Settings → AI Providers), your claude.ai connectors; Isolated
  shares nothing. Containers are defined in `containers.yaml`: packages,
  `setup` steps (shell commands run as root at build time), `brain: shared`
  or `sandboxed`, `mcp` and `connectors: shared` or `none`, `secrets:
  shared` (your `%USERPROFILE%\.secrets` folder, one file per service,
  read-only at `~/.secrets`) and `folders` (more Windows folders at
  `/home/uncli/<name>`, read-only unless `access: rw`; decision 0017). Each
  container lists what's mounted, from where and how, in
  `/etc/uncli/mounts.json` (`$UNCLI_MOUNTS`). Settings →
  Containers edits them (Edit on each, New container, Reset to built-in,
  Delete), writing your `containers.yaml` in the config folder, which adds
  containers or replaces the built-in ones by `id`. Settings says when a
  container no longer matches its config and needs a Rebuild; conversations
  survive rebuilds.

### Your sessions from your phone

UNCLI can serve its sessions to your phone, or another PC, over
[Tailscale](https://tailscale.com) (decisions 0021 and 0022):

1. On the PC: Tailscale running, with HTTPS certificates on in your tailnet
   (admin console, DNS). Settings → Remote access → **Serve sessions to
   paired devices**. It shows the address, such as
   `https://<pc>.<tailnet>.ts.net:9871`, or why there isn't one.
2. **Pair a device** shows a QR code. Scan it with the phone, name the phone,
   and click **Allow** on the PC once it names the machine on your tailnet
   you expect. Settings lists paired devices; Revoke shuts one out at once.
3. On the phone, add the page to the home screen and turn on notifications
   in its Settings (all, approvals only, or off, per phone).

The phone sees the PC's sessions live, sends messages and photos, answers
approvals (the trace says it was answered there), and starts sessions in a
folder it browses on the PC. UNCLI listens only on this PC and its
Tailscale address, never your local network, and only while Remote access
is on. Closing the window keeps UNCLI running in the tray (Open and Quit on
its icon); Settings → General → Start with Windows brings it back after a
reboot. `uncli.exe --quit` quits a running UNCLI.

## Building

Requirements: Node 24, Go 1.26 and the [Wails CLI](https://wails.io) v2.10.1.

```powershell
npm run dev               # run the app with hot reload
npm run build             # build it (app\build\bin\uncli.exe)
npm run build:installer   # and its installer (app\build\bin\uncli-amd64-installer.exe; needs NSIS)
npm run check             # go vet and svelte-check
npm test                  # Go tests (parser over recorded CLI streams) and frontend tests
npm run test:integration  # real CLI: downloads the pinned version, uses your sign-in
npm run test:e2e          # real app and CLI on Windows, driven in headless Edge
npm run website           # landing page on http://localhost:5180
npm run notices           # regenerate THIRD-PARTY-NOTICES.md after changing dependencies
npm run release           # build a release into dist/ (needs RELEASE_VERSION and NSIS)
npm run install-build     # install the built installer on this PC and start UNCLI again, even from a session (app/internal/update/install-build.ps1)
```

The npm scripts are the only entry points; VS Code tasks and CI call the
same ones. `test:integration` and `test:e2e` spend a little of your Claude
usage (Haiku turns) and are not part of `npm test`.

For development, `UNCLI_DATA_DIR` moves the app data elsewhere and
`UNCLI_CLAUDE_BIN` runs a specific Claude binary instead of the managed one.
`npm --prefix app/frontend run dev` runs the UI alone on a mock backend.

| Folder | What |
|---|---|
| `app/` | The app: Go backend (`internal/`), Svelte frontend (`frontend/`), built-in profiles (`config/defaults/`), recorded CLI streams (`testdata/streams/`) |
| `website/` | The landing page, published to GitHub Pages by `.github/workflows/deploy-pages.yml` |
| `docs/` | The build brief, UI conventions and decision records |
| `scripts/` | Build, release and dev helpers, and the end-to-end check |

## Releasing

1. Add a `## <version>` section to [CHANGELOG.md](CHANGELOG.md); it becomes
   the release notes.
2. Run `npm run notices` if dependencies changed, and commit.
3. Tag and push: `git tag v0.1.0-alpha`, then `git push origin v0.1.0-alpha`.

The Release workflow builds the installer with `npm run release` and
publishes it with its checksum, `LICENSE` and the third-party notices. A
tag with a pre-release suffix (`v0.1.0-alpha`, `v1.0.0-rc1`) is published as a
pre-release; the installer itself carries the numeric part (0.1.0).

## Security

See [SECURITY.md](SECURITY.md) to report a vulnerability privately.

## Licence

UNCLI is released under the [MIT licence](LICENSE). The open-source
packages built into it are listed, with their licences, in
[THIRD-PARTY-NOTICES.md](THIRD-PARTY-NOTICES.md). UNCLI doesn't include
any provider's CLI: it downloads each one from its provider, and you use it
under that provider's terms.

Claude and Claude Code are trademarks of Anthropic, Antigravity of Google,
and Grok of xAI. UNCLI is an independent project, not made or endorsed by
any of them.

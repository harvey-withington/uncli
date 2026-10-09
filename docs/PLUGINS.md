# Provider plugins

A provider plugin adds an AI CLI to UNCLI without Go code: a `provider.yaml`
that describes the CLI, run by UNCLI's generic adapter
(`app/internal/adapter/manifest`, decision 0013). UNCLI's built-in providers
(Claude Code, the Antigravity CLI) are Go adapters and don't use it.

A plugin suits a CLI that:

- runs headless and stays up for many turns, reading one JSON message a line
  on stdin and writing one JSON event a line on stdout;
- can be downloaded as a single binary, with a checksum;
- signs in through its own app or terminal (UNCLI checks the sign-in, it
  doesn't perform it).

A CLI that speaks **ACP** (the Agent Client Protocol) needs even less: see
"ACP plugins" below (decision 0014). A CLI that needs more (another control
protocol, one process per turn) needs a Go adapter for now.

## Where plugins live

```
<UNCLI config folder>/providers/<id>/provider.yaml
```

The config folder is `%APPDATA%\uncli` on Windows. The folder's name must be
the manifest's `id`, and the id can't be a built-in's (`claude`,
`antigravity`).

## Trust

A plugin does nothing until the user enables it in Settings → AI Providers.
Settings first shows:
- where its CLI is downloaded from;
- the command it runs;
- whether it switches off the CLI's own permission checks.

Enabling approves the manifest as it is: UNCLI keeps its SHA-256, and a
changed manifest has to be enabled again. A disabled plugin's CLI is never
downloaded or run, not even to check its sign-in.

## The manifest

`app/testdata/providers/antigravity/provider.yaml` is a complete,
working example: the Antigravity CLI as a plugin. The sections:

### Who it is

```yaml
id: example-cli          # lower-case letters, digits and dashes
name: Example CLI        # the CLI, in setup and settings ({cli} in UNCLI's text)
agent: Example           # who acts in a session ({agent})
account: Example         # the account it signs in with ({account})
signin: elsewhere        # its own app or terminal signs it in
```

### install

Builds are pinned: a version, and per platform (`goos/goarch`) an https URL
with a `sha512` or `sha256`. UNCLI downloads into its own cache, never the
user's install. `latest` optionally reads the newest build from a JSON
release manifest: `{platform}` in its URL becomes the name in `platforms`,
and `version`, `binary` and `sha512`/`sha256` name the fields to read.

```yaml
install:
  pinned: 1.2.0
  binary: example
  builds:
    1.2.0:
      windows/amd64: {url: https://…/example-1.2.0.exe, sha512: …}
  latest:
    url: https://…/manifests/{platform}.json
    platforms: {windows/amd64: windows_amd64}
    version: version
    binary: url
    sha512: sha512
```

### env, state

`env` is set for every run; use it to stop the CLI updating itself. `state:
true` gives the CLI a folder of UNCLI's, `{state}` in arguments.
`statePaths` are its bookkeeping folders in it: writing there isn't counted
as changing the user's files.

### launch

`args` always. The others are added when they apply, and can use `{state}`,
`{resume}`, `{id}`, `{model}` and `{effort}`:

```yaml
launch:
  args: ["--state={state}", "--input", "json", "--output", "json"]
  resume: ["--conversation", "{resume}"]   # resuming a conversation
  newId: ["--session-id", "{id}"]           # a new one, if the CLI takes UNCLI's id
  model: ["--model", "{model}"]
  effort: ["--effort", "{effort}"]
  plan: ["--mode", "plan"]                  # the plan permission mode
  approvals: ["--yes-to-all"]               # only with approvals.hook: see below
```

### turn

One line on stdin per message; `{text}` is the message (UNCLI's directives
first). Any JSON shape:

```yaml
turn:
  line: {event: user, message: {content: "{text}"}}
```

### status and models

`signin: device` runs `status.login` hidden and shows the link and code it
prints (Settings → AI Providers → Sign in); the code can be approved on any
device. `status.args` runs the CLI to check its sign-in. `status.login` is how to
start it for the user to sign in: UNCLI opens it in a terminal window
(Settings → AI Providers → Sign in in a terminal) and notices when the
sign-in lands. It's signed in when the
output matches `status.signedIn` (a pattern) or, without one, when it lists
models. `models.line` is a pattern for one model a line: its id, then
optionally its display name.

```yaml
status:
  args: ["models"]
  method: Google
models:
  line: '^([^\s:]+)\t(.+)$'
```

### capabilities

What the CLI can do, as UNCLI's capability names: `partialStreaming`,
`resume`, `usageReporting`, `thinkingEvents`. Plugins can't do live model
switches, interrupts, attachments or slash commands; UNCLI stops the process
to interrupt and starts it again for a new model.

### events

Every output line is matched against every rule in order. A rule applies
when every field in `when` holds (a value, or one of a list). Paths are
dotted (`step_update.tool_info.name`; a number indexes a list). A line no
rule matches is logged as unknown. What a rule can do, in this order:

| Part | What it does |
| --- | --- |
| `session: {id, model, tools, permissionMode}` | The conversation's id (what resume takes) and what the CLI says it runs with |
| `turnStarted: true` | A turn begins (the turn's usage starts from zero) |
| `text: {delta, block, done}` | Streamed text, gathered per `block` until `done` holds, then the block is complete; or `whole` for a complete block |
| `usage: {input, output, cacheRead, cacheWrite}` | Adds the line's tokens to the turn's (several paths are summed) |
| `tool: {id, name, input, done, ok, output, error, denied}` | A tool call: started the first time its `id` (a template, e.g. `"{session}:{step}"`) is seen, finished when `done` holds; `ok` says it worked; an `error` present means it failed, and `denied` is the words that mean it was refused |
| `result: {ok, text, error, errorCode, interrupted, durationSeconds or durationMs, durationTotal, usage, cost, costTotal, denials}` | The turn's end. `usage: turn` uses what the turn's lines added up to; `durationTotal` and `costTotal` mean the values are running totals for the process |

A rule with only `when` marks lines as known that need no event.

### tools

What each tool does, in UNCLI's action kinds (`shell`, `read`, `search`,
`write`, `edit`, `web`, `mcp`, `question`, `plan`, `agent`, `internal`,
`other`). UNCLI decides when to ask from these alone. A tool that isn't
listed is `other`, which asks.

```yaml
tools:
  shell: {dialect: {windows: powershell, other: bash}}
  kinds: {run_command: shell, view_file: read, write_to_file: write}
  prefixes: {"mcp_": mcp}
  command: [CommandLine]            # arguments holding a shell command
  path: [AbsolutePath, TargetFile]  # arguments holding a file
  mcp: {server: [ServerName], tool: [ToolName]}
  summary: [toolSummary, CommandLine]
  touched: {write_to_file: write}   # files the page lists as changed
```

### approvals

Without `approvals`, the CLI keeps its own permission checks, and in
headless mode usually refuses anything that needs permission.

With `approvals.hook`, UNCLI's hook decides every tool call: the same
cards, safe list, prompt levels and unattended mode as the built-in
providers (decision 0012). `files` are written into `{state}` before the
CLI starts; `{hook:<event>}` is the command that runs UNCLI's hook for that
event. `events` say how to read each call and write UNCLI's answers. One
`tool` event is required, and `instructions` optionally gives the session's
system prompt before each model call.

```yaml
approvals:
  hook:
    files:
      "{state}/config/hooks.json":
        uncli:
          PreToolUse:
            - matcher: "*"
              hooks: [{type: command, command: "{hook:pre-tool}", timeout: 86400}]
    events:
      pre-tool:
        kind: tool
        id: "{conversationId}:{stepIdx}"   # the same id as the stream's tool
        name: toolCall.name
        input: toolCall.args
        allow: {decision: allow}
        deny: {decision: deny, reason: "{reason}"}
```

`launch.approvals` (arguments that switch the CLI's own checks off) is only
allowed with a hook. Before relying on it, check against the real CLI that
every way the hook can fail (a crash, a timeout, output that isn't JSON, an
empty answer) refuses the call. `TestAntigravityHookFailsClosed` in
`app/internal/integration` shows how.

## ACP plugins

`protocol: acp` is for CLIs that speak the Agent Client Protocol (Grok
Build's `grok agent stdio`, and other ACP agents). The protocol defines
turns, events and approvals, so the manifest has no `events`, `turn`,
`launch.approvals` or hook:
- the agent asks before each tool call, and UNCLI answers with its "once"
  options (the safe list remembers what you teach it);
- Stop is a real cancel;
- resume is `session/load`.

UNCLI offers the agent no files or terminal, so it uses its own tools.
What an ACP manifest adds:

```yaml
protocol: acp
signin: device                  # or elsewhere
files:                          # written into {state} before it starts
  "{state}/config.toml": |
    [cli]
    auto_update = false
env: {GROK_HOME: "{state}"}     # {state} works in env values
launch:
  args: ["agent", "--no-leader"]
  model: ["--model", "{model}"]
  tail: ["stdio"]               # always last: the subcommand
status:
  args: ["models"]
  signedOut: "(?i)not authenticated"   # a pattern that means signed out
  login: ["login", "--device-auth"]    # prints a link and a code
  deviceUrl: 'https://accounts\.x\.ai/\S+'
  deviceCode: '\b[A-Z0-9]{4}-[A-Z0-9]{4}\b'
acp:
  usage:                        # paths into a prompt's result, if the agent reports usage
    input: _meta.usage.inputTokens
    output: _meta.usage.outputTokens
    cost: _meta.usage.costUsdTicks
    costScale: 0.0000000001
```

`app/testdata/providers/grok/provider.yaml` is the complete Grok Build
plugin; copy that folder to the config folder's `providers/grok` to use it.

## Testing a plugin

1. Record what the CLI prints for a few turns into
   `app/testdata/streams/<id>/<version>/`: plain turns, a resume, tool calls
   with the hook allowing and denying, a sub-agent, an error. For hooks, keep
   what the hook was sent too. `app/testdata/streams/README.md` describes
   the layout.
2. Load the manifest in a test with `manifest.Load` and feed it the fixtures:
   nothing should come out unknown, and the events should be what the
   recording shows. `app/internal/adapter/manifest/compare_test.go` does
   this for the Antigravity manifest, against UNCLI's own adapter.
3. Run it for real: put it in the config folder, enable it, and start a
   session (`TestAntigravityAsAPlugin` in `app/internal/integration`).

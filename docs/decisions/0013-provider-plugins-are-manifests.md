# 0013. Provider plugins are manifests run by a generic adapter

Status: accepted
Date: 2026-10-09

## Context

UNCLI has two providers, Claude Code and the Antigravity CLI, each a Go
adapter compiled into UNCLI. Harvey (2026-10-09) wants a plugin system so
a new CLI needs as little code as possible, without risking the two that
work: "I don't want to introduce bugs into a working platform provider
while I'm using it."

Writing the Antigravity adapter showed that most of a provider is the same
shape every time. What differs is data:
- the commands and flags;
- the JSON field names on the stream;
- the tool names and which argument holds a command or a path;
- the release URLs and checksums;
- what a hook is sent and what it answers.

These were weighed:
1. **Code plugins in another process** (a JSON-RPC protocol, like LSP or
   MCP). Flexible and any language, but every CLI is a program to write,
   ship and trust.
2. **Go plugins or WASM modules.** Go's plugin package doesn't work on
   Windows. WASM is sandboxed but heavy, and still code per CLI.
3. **A declarative manifest run by a generic adapter.** A CLI is described
   as data; the engine is written and tested once.

## Decision

Option 3, with room for an escape hatch later.

- **A plugin is a folder.** `<config>/providers/<id>/provider.yaml`. It
  describes:
  - how to install the CLI: pinned builds with checksums, and the latest
    from a release manifest;
  - its environment;
  - how to start it: new, resume, model, effort, a state folder of its own;
  - how a turn is written;
  - rules that map each output line to UNCLI's events;
  - what each tool does;
  - how sign-in is checked and models are listed;
  - how approvals work: none, or a hook (decision 0012);
  - its capabilities.
- **The engine is just another adapter.** `internal/adapter/manifest`
  implements `core.Adapter` (and `HookAdapter`, `StatePather` and model
  listing) from a manifest. It joins the provider list beside the built-in
  adapters, which are not changed.
- **Plugins can't hurt the built-ins.**
  - A plugin that fails to load or validate is listed in AI Providers with
    its error, and nothing else happens.
  - Its id can't be a built-in's.
  - Sessions keep the provider they were created with.
- **Plugins start disabled.** AI Providers shows what an enabled plugin
  would run, and whether it switches off its CLI's own permission checks.
  The user enables it, and the approval is tied to the manifest's hash: a
  changed manifest needs approving again.
- **Switching off a CLI's checks needs a gate.** A manifest that switches
  them off must route every tool call through UNCLI's hook (matcher `*`),
  or it is refused when loaded (decision 0012's rule).
- **The engine is proven against a built-in.** Antigravity is also written
  as a manifest, kept with the tests. A comparison test runs every recorded
  Antigravity fixture through both the Go adapter and the manifest, and
  requires the same events, hook answers, commands and tool actions.
  Moving a built-in onto a manifest is optional later work, done side by
  side and behind that test.

## Consequences

- A new CLI that streams JSON lines can be added without Go code: a
  manifest, its fixtures, and the conformance test.
- The rules can only express what the engine knows:
  - field matches;
  - text deltas accumulated per block;
  - tool start and end deduplicated by id;
  - usage summed per turn, or taken from the result;
  - durations per turn, or from a running total.

  A CLI that needs more (a control protocol like Claude Code's, an
  unusual sign-in) is either a Go adapter or waits for the escape hatch
  (a transformer process).
- Plugins only run CLIs downloaded and checked by UNCLI. One that names a
  binary already on the user's machine is "at their own risk", as for the
  built-ins.
- Containers, quick tasks and import stay with the first provider until
  the core is generalised (the plugin card's later slices).

# 0010. Hosts theme UNCLI through public tokens

Status: accepted
Date: 2026-10-07

## Context

UNCLI is meant to be embeddable in other applications, and an embedded UNCLI
has to look like part of whatever hosts it. The first host will be BRUV.

The BRUV card "Theme tokens shared with BRUV" proposed keeping UNCLI's
tokens in step with BRUV's by copying values across. That ties UNCLI to one
host. Each new host would need another copy, and every change to a host's
palette would mean a change to UNCLI.

UNCLI's tokens (`--bg`, `--accent`, …) are unprefixed custom properties on
`:root`, and its dark theme keys on `data-theme` on `<html>`. Inside a host
page, both would collide with the host's own (many apps use `--bg`,
`--accent` and `data-theme`).

## Decision

Harvey, 2026-10-07: UNCLI keeps its own look and publishes a theme override
contract. A host supplies values through that contract, and UNCLI never
refers to any particular host.

- **Public tokens.** The contract is a set of `--uncli-*` custom properties:
  colours (`--uncli-bg`, `--uncli-surface`, `--uncli-text`, `--uncli-accent`,
  …), shadows, scrim, the chart hue, fonts and radii. Each internal token
  reads its public one and falls back to UNCLI's own value
  (`--bg: var(--uncli-bg, #f7f6f3)`), so with nothing set UNCLI looks exactly
  as before. The list is in `docs/UI-CONVENTIONS.md`, and a test keeps it in
  step with `app.css`.
- **Derived tokens follow.** The soft fills (`--accent-soft`, `--danger-soft`,
  `--warning-soft`) are mixed from their base colour (`color-mix`), so a host
  that sets the accent gets a matching soft fill. UNCLI's own section-kind
  colours, type tint lightness and code highlighting stay its own: they're
  part of what pages mean, not the host's brand.
- **Scheme is UNCLI's own attribute.** UNCLI always sets the scheme it
  resolved (the user's pick, or the system's) on its root as
  `data-uncli-scheme="light|dark"`. It replaces `data-theme`. Theme values
  key on it, so one theme can carry light and dark values.
- **Values, not stylesheets.** A theme supplies values for tokens in the
  contract and nothing else. Unknown names are ignored with a warning.
  A value holding `;`, braces, angle brackets or `url(` is rejected, so a
  theme can't add rules or fetch anything.
- **Two ways in.**
  - The desktop app reads an optional `theme.yaml` in UNCLI's config folder
    (`light:` and `dark:` maps of token names without the prefix) and applies
    it at start and on "Reload theme file". This is how a host's look can be
    tried today. It also lets a user restyle UNCLI.
  - An embedding host will set the `--uncli-*` properties on the element
    UNCLI mounts in, usually as references to its own tokens
    (`--uncli-bg: var(--bg-base)`), and tell UNCLI its scheme through the
    embed API. When embedding is built, the internal tokens move from
    `:root` to UNCLI's root element, so the host's values reach them.
- **The host knows the guest, never the reverse.** A host's mapping of its
  tokens to UNCLI's lives in the host's repository. BRUV's mapping is BRUV's
  file, and nothing in UNCLI names BRUV.

## Consequences

- The internal token names can change freely. Only the `--uncli-*` names are
  a public contract, so renaming or removing one is a breaking change for
  hosts and goes in the README's notes.
- A host chooses its colours, and UNCLI can't check every host palette for
  contrast. Section-kind colours were validated against UNCLI's own
  surfaces, so a host with very different surfaces may want to check them.
- `?theme=light|dark` in the browser mock and the user's theme setting still
  work. They now set `data-uncli-scheme`.

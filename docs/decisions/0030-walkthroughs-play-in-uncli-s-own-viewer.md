# 0030. Walkthroughs play in UNCLI's own viewer; reveal.js is for export

Status: accepted (supersedes 0029's "reveal.js plays it")
Date: 2026-11-08

## Context

Decision 0029 played a page's walkthrough with reveal.js in a sandboxed
iframe. On the phone that went wrong in several ways (Harvey, 2026-11-08):

- reveal.js draws a fixed-size slide and scales it to the screen, like a
  projector. A phone wants text that stays readable and reflows: headings
  came out huge, code tiny, and each screen shape needed its own slide size.
- Turning the phone blanked the deck until reveal.js laid it out again, and
  a landscape phone got the desktop dialog with the slide off to one side.
- The frame inherits the page's content security policy, and the phone
  app's refuses inline scripts, so reveal.js's script had to become a file
  of its own, made by a build plugin.

Of what reveal.js offered, the walkthrough used its keys and slide
changes; its step highlights had already been replaced with UNCLI's own.
Its overview and print view weren't needed yet.

## Decision

- **UNCLI's own viewer plays walkthroughs.** A Svelte component, in the
  chat's own page (no iframe): a row of full-width slides that snaps to
  each one, so a phone swipes between them natively, with arrow keys, Page
  Up and Down, Home and End, previous and next buttons, a "3 / 8" counter
  and a full screen button. Slides use UNCLI's fonts, sizes and colours, and
  reflow at any size, turned or not.
- **reveal.js isn't part of UNCLI.** The dependency, the iframe and the
  build plugin go.
- **reveal.js Markdown is an export format, later.** A walkthrough exported
  as Markdown with `---` between slides opens as a reveal.js deck in its
  own tools (the VS Code extension, reveal.js's Markdown plugin), where a
  fixed slide suits presenting.

Everything else in 0029 stands: the recorded changes, the deck built from
them, the button, and Explain to come.

## Consequences

- The walkthrough works the same in the app, on the phone and in a host's
  chat, with no frame, no content security policy workaround and no extra
  download.
- No overview of all the slides, no print view and no transitions beyond
  scrolling; any of these is UNCLI's to add if it's missed.
- Export is a separate piece of work, with reveal.js Markdown as its first
  format.

# 0019. Markdown renders cleaned HTML and task lists

Status: accepted
Date: 2026-10-23

## Context

UNCLI rendered Markdown with markdown-it and raw HTML off, because model
output is untrusted and the app window holds the bridge to the Go side: a
script there could answer approvals or open files. So HTML showed as its
source, and task lists ("- [ ] thing") as brackets. With the File tab
showing a repo's own Markdown (READMEs with `<details>`, `<kbd>`, `<img>`;
TODO lists the agent ticks off), both looked broken (Harvey, 2026-10-23).
BRUV renders the same Markdown with marked and lets HTML through as it is,
which is fine for a user's own cards but not for model output.

## Decision

- markdown-it stays (its line maps give every block its source), with
  raw HTML on and every rendered block cleaned by DOMPurify before it
  reaches the page. DOMPurify keeps the HTML profile (no SVG or MathML)
  and drops scripts, event handlers, `style` tags and attributes, forms
  and their controls, frames, objects, `<dialog>`, `<template>`,
  `<link>`, `<meta>`, `<base>`, ids, names, `target` and `tabindex`.
  Only UNCLI's own classes are kept, so HTML can't borrow the app's
  styles. URLs may be web, mail, phone or `file:` links, as Markdown
  links may; `javascript:` and the like are removed. Every link, Markdown
  or HTML, goes through the click handler and never navigates the window.
- Task list items show a checkbox, read only, ticked for `[x]`; ticked
  items are muted.
- HTML that wraps Markdown (`<details>`, blank line, Markdown, blank line,
  `</details>`) spans several of markdown-it's top-level blocks. A block
  of HTML that leaves a tag open takes the blocks after it, up to the tag
  that closes it (or the end), so it renders and copies as one.

## Consequences

- Answers, artifacts and watched files render collapsible sections, key
  caps, images and task lists as GitHub does.
- Images from the web load, as Markdown images already did.
- A code fence inside such an HTML block renders as plain code, without
  highlighting or its own copy button, and headings inside it don't make
  the outline.
- An answer that opens a tag it never closes renders the rest as one
  block. While an answer streams, the block grows until the tag closes.
- Ticking a box doesn't write to the file; the agent edits it.

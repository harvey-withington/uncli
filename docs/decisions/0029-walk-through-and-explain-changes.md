# 0029. UNCLI walks through a turn's changes as a slide show, and explains them on request

Status: accepted; superseded in part by 0030 (UNCLI's own viewer plays walkthroughs; reveal.js is for export)
Date: 2026-11-07

## Context

With an agent writing the code, the hard part is understanding what it
did. Today a page lists the files a turn changed, with counts, and links
to open them; the changes themselves are spread through a long answer, or
only in the files. Harvey (2026-11-06) asked for a slide show: the code a
turn changed, one part at a time, formatted and coloured, with an LLM's
explanation beside it.

Designing it settled three things:

- **The code is most of the value; the explanation is "the icing"**
  (Harvey, 2026-11-07). A walkthrough of the changes needs no LLM.
- **An explanation shouldn't resend the conversation.** A turn in the
  session, or a fork of it, sends the whole context again, and caching
  only helps shortly after the turn. The changes, the page's question and
  its answer are usually enough.
- **A tool call that waits for the user on each step** keeps the turn
  open while they read, and may time out. The whole walkthrough is made
  at once, and UNCLI plays it back.

A page doesn't keep its changes today: `TouchedFile` has paths and counts,
and the stored trace keeps a tool call's summary and output, not its
input.

## Decision

- **UNCLI records each turn's changes.** Each adapter turns its CLI's
  successful edit and write tool calls into UNCLI's own change records
  (file, old text, new text), and the session stores them with the page,
  bounded in size. Changes made by commands aren't recorded: they're
  listed as "changed by a command" with no diff. Pages from before this
  have no walkthrough.
- **Walk through the changes, without an LLM.** A button above a page's
  changed files opens a slide show of its hunks (a diff per slide, with
  the file and line numbers), built by UNCLI from the records. It costs
  nothing.
- **Explain them on request, in a separate small call.** Not a turn in
  the session and not a fork. UNCLI sends the hunks (with ids), the
  page's question and answer, and earlier pages only when asked. The LLM
  answers with steps naming hunk ids (and ranges within them), a title
  and an explanation, as data; UNCLI checks each against the hunks and
  never takes code from the LLM. Options, remembered, in a small dialog
  behind the button: the model (the session's by default), the context
  (this page, the last N pages, or the whole session) and the detail
  (brief or thorough), with a rough size before it starts. The
  explanation is stored on the page.
- **reveal.js plays it,** bundled (MIT) and loaded when a deck first
  opens, in a sandboxed frame in a large dialog, with a button to go full
  screen. Code is coloured by Shiki, and
  UNCLI marks each step's lines itself (reveal.js fragments carrying the
  lines), since reveal.js's own line highlights need highlight.js.
- **Every host's `<uncli-chat>` has it.** A host can turn Explain off; the
  walkthrough costs nothing. In a host, the dialog stays inside the
  chat's box, and full screen happens only when the user asks. Both are experimental at first.
- **Later, by the same means:** explaining a selection in the File tab,
  and UNCLI's own tool for the LLM ("explain login.js"), each deck made
  from steps that UNCLI checks.

## Consequences

- Pages get bigger by their changes' text. The size limit keeps a large
  rewrite from swelling the database; past it, files are listed without
  their text.
- Each adapter gains a mapping from its CLI's edit tools to change records,
  with fixtures, as for any new behaviour.
- An explanation knows the page, and earlier pages only when asked, not
  the whole conversation by default. A change whose reasons go back a long
  way gets a thinner explanation unless the user widens the context.
- reveal.js is a new dependency, in the notices, kept out of the bundle
  until a deck opens.
- What gave way: a step-by-step tool call that waits for the user, and
  explaining from the session itself (or a fork of it).

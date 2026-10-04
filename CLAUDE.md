# UNCLI

AI CLIs if you're not a CLI guy. Pronounced "un-clee".

Before changing code here, read the project conventions in
`.claude/skills/conventions/SKILL.md` (or invoke the `conventions` skill):
architecture rules, the frontend contract in `docs/UI-CONVENTIONS.md`, the
checks to run and the commit message format.

- Requirements live on the UNCLI project on BRUV; its id is in Claude's memory
  for this project. `plan/` is a private scratchpad (a separate repo, ignored
  here): nothing there is a requirement until it is a card on the project.
- `docs/BRIEF.md` is the initial handover: architecture, seams, event model,
  data model and the phased roadmap. Read it before structural work. It is
  frozen (final as of 2026-10-04) and doesn't track the current state, so
  don't edit it for new work. Features are fleshed out on their BRUV cards as
  they're built, architecture-shaping choices go in `docs/decisions/`, and new
  planning goes in a new file (an amendment or a feature design) on the card.
- `npm run dev | build | check | test` are the entry points for every stack.
- To see a UI change rendered, use the `run-app` skill.

## Working rules

- Follow the "Working rules for the builder" section of `docs/BRIEF.md`
  alongside the conventions skill. If the two ever disagree, stop and flag it
  rather than picking one.
- Work one task at a time and tick off the phase's definition of done as items pass.
- If the CLI behaves differently from what the brief assumes, record it on the
  relevant BRUV card (and in a decision record if it changes the design)
  before changing the code.
- Never use `--dangerously-skip-permissions` or bypass mode in any profile default.

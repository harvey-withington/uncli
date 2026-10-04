# 0006. Helper models are app-level and may call an API

Status: accepted
Date: 2026-10-05
Refines: the brief's "pure CLI" backend rule

## Context

The brief said UNCLI never calls a model API directly: every model runs
behind a CLI. That rule exists for the session models (chat, co-work and
coding). Their CLI lets the user spend a monthly subscription allowance
instead of paying per token, and it gives sessions one code path.

UNCLI also uses small helper models outside any conversation. The
quick-task model writes page summaries and judges commands UNCLI can't
place. A decision model (Jev, or a self-hosted Kev) is planned for
fixed-answer judgements such as safe or unsafe. Today the quick-task model
runs through the session provider's CLI. A decider is an HTTP API with no
CLI. Read literally, the rule ruled deciders out, and it tied helpers to a
session's provider.

Harvey, 2026-10-05: "we decided to only support CLIs for the main session
LLM … A decider or a quick model do not need to be part of a session … The
main benefit of the CLI is being able to use monthly session allocations
rather than API tokens … For Jev, and other decision models, that's not an
issue because it's cheap API pricing by default. Similarly, quick LLM models
are cheap enough to use API pricing in most cases."

## Decision

- **Pure CLI applies to session models only.** A session's model never runs
  through a direct API call.
- **Helper models are app-level.**
  - The quick-task model and the decision model belong to the app and are
    shared by every session, whatever runs the session (a CLI today,
    possibly not later).
  - Each is a preference of its own: provider or endpoint, model, and
    version.
  - Neither is tied to a session's provider.
- **Helpers may call an API directly, with their own credentials**: an API
  key the user gives for that purpose, or none for a local model. They
  never reuse a CLI's credentials outside the official binary.
- **A CLI-backed helper stays an option.** The quick-task model keeps
  running through `TextTasker` until an API-backed one is added.

## Consequences

- A decider (hosted Jev, or Kev on the home server) can be added without
  breaking a rule, as an app-level service behind a provider-neutral seam
  in `core`.
- Helpers need their own settings and credential storage, separate from
  the CLI's sign-in. A local-only option keeps data at home.
- Helper calls cost API tokens, not subscription allowance. That is
  acceptable at their size.
- Sessions keep one code path: the CLI adapter.

# Security

UNCLI stands between an AI agent and your computer: it decides which tool
calls the agent may make, and containers decide what the agent can see. A
bug that weakens either is a security bug, and reports are welcome.

## Reporting a vulnerability

Report it privately through GitHub:
[Report a vulnerability](https://github.com/harvey-withington/uncli/security/advisories/new).
Please don't open a public issue for it.

Say what you did, what happened and what you expected, with the UNCLI
version (Windows Settings → Apps → Installed apps) and, if it matters, the
AI provider and its CLI version (Settings → AI Providers). You'll get an answer within a week, and credit in the release
notes for the fix if you want it.

## What counts

For example:

- a tool call that runs without the approval its profile and Prompt me
  setting require, or a hook failure that lets a call through instead of
  refusing it;
- a container that can reach files outside the session's folder, start
  Windows programs or gain root;
- a provider plugin that runs or downloads anything before it is enabled,
  or after its manifest changed;
- a CLI download that isn't checked against its pinned checksum;
- a secret UNCLI keeps (a container's sign-in token, the decision model's
  API key) written anywhere other than the Windows Credential Manager, or
  passed to the UI.

What an AI provider's own CLI or service does is for that provider; UNCLI
can only limit what it allows the CLI to do.

## Supported versions

Fixes go into the latest release only.

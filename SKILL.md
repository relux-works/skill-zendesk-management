---
name: zendesk-management
description: Agent skill and companion CLI for working with Zendesk Support data, including auth bootstrap, tickets, comments, users, organizations, and setup flows for the local zendesk-mgmt tool.
---

# Zendesk Management

Use this repo when the user needs to:

- bootstrap or manage `zendesk-mgmt`
- configure Zendesk access for local agent work
- implement or use Zendesk read workflows for tickets, comments, users, and organizations

## Quick Start

Set access once:

```bash
zendesk-mgmt auth set-access --organization acme --email agent@acme.com --token YOUR_TOKEN
```

Inspect the current auth state without printing the secret:

```bash
zendesk-mgmt auth whoami --organization acme
zendesk-mgmt auth resolve --organization acme
```

`auth whoami` performs a live Zendesk auth check by default. Use `--check=false` when you only want to inspect local storage state.

Clean up stored access:

```bash
zendesk-mgmt auth clean --organization acme
```

## References

Read these files as needed:

- `references/auth-config.md` for storage modes and config paths
- `references/zendesk-api.md` for official Zendesk auth, pagination, and endpoint notes
- `references/mvp-contract.md` for the current CLI and setup contract

## Implementation Rules

- Keep the user-facing auth UX under `auth/*`.
- On macOS, default auth storage is Keychain.
- On Windows-friendly flows, default auth storage is the standard global config under `os.UserConfigDir()/zendesk-mgmt/auth.json`.
- Keep the CLI compatible with the `agent-facing-api` pattern.
- Prefer table-driven tests and `httptest`-style local verification over live-network tests.

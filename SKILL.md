---
name: zendesk-management
description: Agent skill and companion CLI for working with Zendesk Support data, including auth bootstrap, tickets, comments, users, organizations, and setup flows for the local zendesk-mgmt tool.
---

# Zendesk Management

Use this repo when the user needs to:

- bootstrap or refresh `zendesk-mgmt`
- configure Zendesk access for local agent work
- search and read Zendesk tickets, comments, and attachments
- inspect auth state or verify live Zendesk access
- implement or extend the read-only Zendesk facade

## Working Surface

The companion CLI has three main read surfaces:

- `auth` for setup and credential inspection
- `q` for structured reads through the mini DSL
- `grep` for quick ticket discovery through Zendesk Search
- `attachment download` for saving a binary attachment to disk

Prefer the installed binary:

```bash
zendesk-mgmt version
```

If you are working from the source repo and need to refresh the installed copy:

```bash
./setup.sh --install-only
```

## Auth Bootstrap

Set access once per organization:

```bash
zendesk-mgmt auth set-access --organization acme --email agent@acme.com --token YOUR_TOKEN
```

Inspect auth state without printing the secret:

```bash
zendesk-mgmt auth whoami --organization acme
zendesk-mgmt auth resolve --organization acme
```

`auth whoami` performs a live Zendesk auth check by default. Use `--check=false` when you only want to inspect local storage state.

Clean up stored access:

```bash
zendesk-mgmt auth clean --organization acme
```

Storage behavior:

- on macOS, auth defaults to Keychain
- on Windows-friendly flows, auth defaults to the global config under `os.UserConfigDir()/zendesk-mgmt/auth.json`

## Typical Read Flow

Use this order for most support investigations:

1. Find the ticket with `grep` or `search(...)`
2. Read the ticket with `ticket(id)`
3. Pull the conversation with `ticket_comments(ticket_id=...)`
4. Inspect attachment refs from `ticket(id)` or `ticket_attachments(ticket_id=...)`
5. Download the exact attachment if needed

Start with a quick search:

```bash
zendesk-mgmt grep 'invoice failed' --organization acme --type ticket --limit 10 --format compact
zendesk-mgmt q 'search(query="type:ticket status:open invoice failed", page=1, per_page=5) { overview }' --organization acme --format compact
```

Read one ticket:

```bash
zendesk-mgmt q 'ticket(12345) { default }' --organization acme --format compact
zendesk-mgmt q 'ticket(12345) { full }' --organization acme --format json
```

`ticket(id)` `default` and `overview` include a derived `attachments` field with compact refs like `15759358677263 logs.zip`.

Read the conversation:

```bash
zendesk-mgmt q 'ticket_comments(ticket_id=12345, limit=20) { overview }' --organization acme --format compact
```

Inspect attachments:

```bash
zendesk-mgmt q 'ticket_attachments(ticket_id=12345, limit=20) { overview }' --organization acme --format compact
zendesk-mgmt q 'attachment(15759358677263) { default }' --organization acme --format compact
```

Download an attachment:

```bash
zendesk-mgmt attachment download 15759358677263 --organization acme --destination ~/Downloads/
zendesk-mgmt attachment download 15759358677263 --organization acme --destination /tmp/logs.zip --force
```

`--destination` accepts either a directory or a full file path.

## Output Modes

Use `--format compact` for agent-facing investigation:

- smaller output
- better for quick scanning
- better default for search, ticket reads, and comments

Use `--format json` when:

- piping into another tool
- inspecting full payloads
- comparing raw fields
- validating schema behavior

Discover the supported query surface with:

```bash
zendesk-mgmt q 'schema()' --organization acme --format json
```

## References

Read these files as needed:

- `README.md` for the current user-facing CLI surface
- `references/auth-config.md` for storage modes and config paths
- `references/zendesk-api.md` for official Zendesk auth, pagination, and endpoint notes
- `references/mvp-contract.md` for the current CLI and setup contract
- `references/agent-facing-facade-spec.md` for the read-facade contract and entity/preset coverage

## Implementation Rules

- Keep the user-facing auth UX under `auth/*`.
- On macOS, default auth storage is Keychain.
- On Windows-friendly flows, default auth storage is the standard global config under `os.UserConfigDir()/zendesk-mgmt/auth.json`.
- Keep the CLI compatible with the `agent-facing-api` pattern.
- Prefer ticket-centric workflows first; user and organization reads are supporting lookups.
- Do not add mutation commands without a separate spec.
- Prefer table-driven tests and `httptest`-style local verification over live-network tests.

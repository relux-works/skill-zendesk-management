# MVP Contract

This file fixes the first implementation contract for the Zendesk skill and CLI.

## Goal

Ship a small read-only Zendesk CLI that an agent can use without wasting tokens on verbose human output.

## Credential Contract

The first auth shape is:

- `instance_url`: `https://{subdomain}.zendesk.com`
- `email`
- `api_token`
- `organization`
- `auth_source`: `keychain` or `env_or_file`
- `auth_type`: `api_token` for the current CLI shape

Storage rule:

- support two explicit token sources:
  - `keychain`
  - `env_or_file`
- provide a user-facing `auth set-access --organization ORG --email EMAIL --token TOKEN` command
- in `keychain` mode, use service name `zendesk-mgmt` and store by derived `instance_url`
- in `env_or_file` mode, resolve `ZENDESK_EMAIL` plus `ZENDESK_API_TOKEN` first, then the global JSON file at `os.UserConfigDir()/zendesk-mgmt/auth.json`
- the global JSON file is sectioned by organization, and each profile currently contains `email`, `api_token`, and `auth_type`

Preferred implementation pattern:

- use `github.com/zalando/go-keyring`
- mirror the proven shape already used in local Atlassian CLIs in this workspace

## CLI Shape

Binary name:

- `zendesk-mgmt`

Primary agent interface:

- `zendesk-mgmt q 'schema()'`
- `zendesk-mgmt q 'ticket(123){overview}'`
- `zendesk-mgmt q 'ticket_comments(123){default}'`
- `zendesk-mgmt q 'search(query=\"type:ticket status<solved\", take=25){overview}'`
- `zendesk-mgmt q 'user(456){full}'`
- `zendesk-mgmt q 'organization(789){full}'`

Secondary human/auth interface:

- `zendesk-mgmt auth set-access --organization acme --email agent@acme.com --token ...`
- `zendesk-mgmt auth whoami --organization acme`
- `zendesk-mgmt auth resolve --organization acme`
- `zendesk-mgmt auth clean --organization acme`
- `zendesk-mgmt version`

Local text search:

- `zendesk-mgmt grep "pattern"`

Write mutations:

- deferred until read contract, pagination, and auth are stable

## Query DSL Rules

Following `agent-facing-api`, the read surface should use:

- one `q` entrypoint
- field projection
- field presets
- schema introspection via `schema()`
- batching via `;`
- explicit output mode selection

Initial operations:

- `schema()`
- `ticket(id)`
- `ticket_comments(ticket_id)`
- `search(query=..., type=..., take=..., page_after=...)`
- `user(id)`
- `organization(id)`
- `users(take=..., page_after=...)`
- `organizations(take=..., page_after=...)`

Initial presets:

- `minimal`
- `default`
- `overview`
- `full`

Output modes:

- `--format json` for piping, debugging, and humans
- `--format compact` for agent use

## Repo Scaffold

The first code layout should be:

```text
cmd/zendesk-mgmt/
internal/config/
internal/zendesk/
internal/query/
internal/output/
testdata/
references/
```

Package responsibilities:

- `internal/config`: keychain store, instance config, auth resolution
- `internal/zendesk`: request builder, auth transport, pagination, typed API calls
- `internal/query`: DSL parser, operation registry, field selection, batching
- `internal/output`: JSON and compact formatting

## Skill Setup Pattern

Follow the `skill-project-management` setup pattern rather than inventing a one-off installer.

Canonical expectations:

- one source-checkout setup entrypoint
- build the CLI from source with embedded version metadata
- install the binary into the user-local bin directory
- copy a degitized installed skill artifact into `~/.agents/skills/zendesk-management`
- refresh `~/.claude/skills/` and `~/.codex/skills/` links to the installed skill copy
- write install state for future self-update or installer refresh logic
- provide an `--install-only`-style safe reinstall mode
- verify that the installed binary on PATH is the expected fresh binary, not a stale one

For this repo, the setup work must be explicitly cross-platform:

- `macOS arm64`
- `macOS x86_64`
- `Windows x86_64`
- `Windows arm64`

Expected release/install shape:

- local source-checkout installer for developer setup
- release artifacts for supported platforms
- cross-platform build config, likely via `goreleaser`
- Windows needs an automatic installer path too, not just a manually unpacked zip

Suggested implementation split:

- `scripts/setup.sh` for macOS local setup from source
- `scripts/setup.ps1` for Windows local setup from source or release artifacts
- release archives for `darwin/arm64`, `darwin/amd64`, `windows/amd64`, and `windows/arm64`

## Testing Strategy

Use `go-testing-tools` principles even though the MVP is a CLI, not a TUI.

Required test layers:

- table-driven parser tests for DSL syntax and parameter validation
- `httptest.Server` tests for auth headers, pagination, and 429 retry behavior
- golden tests for compact output formatting
- fixture-based tests for Zendesk ticket, comment, user, and organization payload decoding

What not to do:

- no live-network tests in the default suite
- no secrets in fixtures
- no tests that depend on a real Keychain entry; inject storage interfaces and fake them

If a TUI or interactive dashboard appears later, add `tuitestkit` at that point rather than prematurely.

## Definition Of Done For MVP Foundation

This contract is complete when:

- keychain-only auth is implemented for one Zendesk instance
- `q schema`, `q ticket`, `q ticket_comments`, `q search`, `q user`, and `q organization` work
- JSON and compact output both exist
- tests cover auth, parser behavior, pagination, rate-limit handling, and compact rendering

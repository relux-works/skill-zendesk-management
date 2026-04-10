# skill-zendesk-management

Repository for a Zendesk-focused agent skill and companion CLI.

Initial scope:
- ticket search and fetch
- ticket comments and conversation export
- user and organization lookup
- agent-oriented workflows and references

Project workflow:
- board state lives in `.task-board/`
- board runtime config lives in `task-board.config.json`
- local project instructions live in `AGENTS.md`
- official API notes and the local MVP contract live in `references/`

## Setup

Canonical setup entrypoints:

```bash
./setup.sh
./setup.sh --install-only
```

On Windows:

```powershell
.\setup.ps1
.\setup.ps1 -InstallOnly
```

What setup does:

- builds `zendesk-mgmt`
- embeds version, commit, and build-date metadata into the binary
- installs the binary into the user-local bin dir
- copies a degitized installed skill artifact into `~/.agents/skills/zendesk-management`
- refreshes `~/.claude/skills/zendesk-management` and `~/.codex/skills/zendesk-management`
- writes install-state metadata for future refresh/update flows
- verifies the installed binary via `zendesk-mgmt version` and `zendesk-mgmt auth config-path`

Supported setup/release matrix:

- `macOS arm64`
- `macOS x86_64`
- `Windows x86_64`
- `Windows arm64`

## Auth Config

`zendesk-mgmt` now has a minimal auth-config scaffold with two token sources:

- `keychain`
- `env_or_file`

Default behavior:

- macOS defaults to `keychain`
- Windows-friendly flows default to `env_or_file`

Current global JSON config schema:

```json
{
  "profiles": {
    "acme": {
      "email": "agent@acme.com",
      "api_token": "your-token-here",
      "auth_type": "api_token"
    }
  }
}
```

Useful commands:

```bash
go run ./cmd/zendesk-mgmt auth config-path
go run ./cmd/zendesk-mgmt version
go run ./cmd/zendesk-mgmt auth set-access --organization acme --email agent@acme.com --token YOUR_TOKEN
go run ./cmd/zendesk-mgmt auth whoami --organization acme
go run ./cmd/zendesk-mgmt auth whoami --organization acme --check=false
ZENDESK_EMAIL=agent@acme.com ZENDESK_API_TOKEN=token go run ./cmd/zendesk-mgmt auth resolve --source env_or_file --organization acme
go run ./cmd/zendesk-mgmt auth resolve --source keychain --organization acme
go run ./cmd/zendesk-mgmt auth clean --organization acme
```

`auth whoami` now performs a live Zendesk auth probe by default using the stored credentials and `GET /api/v2/users/me.json`. Use `--check=false` for storage-only inspection.

See `references/auth-config.md` for the concrete path and resolution rules.

## Query Facade

The agent-facing facade now lives behind two CLI entrypoints:

- `q` for structured DSL reads
- `grep` for scoped text discovery through Zendesk Search

Examples:

```bash
go run ./cmd/zendesk-mgmt q 'schema()' --organization acme --format compact
go run ./cmd/zendesk-mgmt q 'ticket(12345) { overview }' --organization acme --format json
go run ./cmd/zendesk-mgmt q 'ticket(12345) { default }' --organization acme --format compact
go run ./cmd/zendesk-mgmt q 'user(67890) { minimal }; organization(12) { default }' --organization acme --format compact
go run ./cmd/zendesk-mgmt q 'search(query="type:ticket status:open", page=1, per_page=5) { overview }' --organization acme --format compact
go run ./cmd/zendesk-mgmt grep 'invoice failed' --organization acme --type ticket --limit 10 --format compact
```

Current `q` operations:

- `schema()`
- `ticket(id)`
- `tickets(after=?, limit=?, page=?, per_page=?)`
- `ticket_comments(ticket_id, after=?, limit=?, page=?, per_page=?)`
- `attachment(id)`
- `ticket_attachments(ticket_id, after=?, limit=?, page=?, per_page=?)`
- `user(id)`
- `users(after=?, limit=?, page=?, per_page=?, role=?)`
- `organization(id)`
- `organizations(after=?, limit=?, page=?, per_page=?)`
- `organization_memberships(organization_id=?, user_id=?, after=?, limit=?, page=?, per_page=?)`
- `search(query, include=?, sort_by=?, sort_order=?, page=?, per_page=?)`
- `search_count(query)`
- `search_export(type, query, after=?, limit=100)`

Supported formats:

- `json`
- `compact`

Attachment download:

```bash
go run ./cmd/zendesk-mgmt attachment download 498483 --organization acme --destination ~/Downloads/
go run ./cmd/zendesk-mgmt attachment download 498483 --organization acme --destination /tmp/file.bin --force
```

The download flow first resolves attachment metadata through Zendesk API, then
fetches `content_url`. Zendesk auth is only attached when the download host
matches the Zendesk instance host, which avoids leaking credentials to
externally hosted attachment URLs.

`ticket(id)` `default` and `overview` presets now include a derived
`attachments` field with compact refs (`id + file_name`) collected from the
ticket comments, so agents can discover downloadable attachments before
calling `attachment download`.

See `references/agent-facing-facade-spec.md` for the contract and implementation plan.

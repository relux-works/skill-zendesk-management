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

# Repository Guidelines

## Project Structure & Module Organization
This repository is for a Zendesk-focused agent skill plus a companion Go CLI.

- `references/`: official API notes and local implementation contracts. Read these before changing auth or command design.
- `cmd/zendesk-mgmt/`: future Cobra entrypoint for the CLI binary.
- `internal/config/`: credential loading, keychain persistence, config precedence.
- `internal/zendesk/`: HTTP client, typed API operations, pagination, rate-limit handling.
- `internal/query/`: agent-facing query DSL (`q`) and scoped search helpers (`grep`).
- `internal/output/`: JSON and compact/LLM-readable renderers.
- `testdata/`: golden fixtures, recorded payloads, compact-output snapshots.
- `.task-board/`: source of truth for work tracking. Do not edit board files manually.

## Architecture Rules
- Auth is keychain-first. Do not store Zendesk secrets in git, `.env`, or `task-board.config.json`.
- The implemented token sources are `keychain` and `env_or_file`.
- `env_or_file` resolves `ZENDESK_ACCESS_TOKEN` first, then the global JSON config under `os.UserConfigDir()/zendesk-mgmt/auth.json`.
- The intended user-facing credential bootstrap is `zendesk-mgmt auth set-access --organization ORG --email EMAIL --token TOKEN`.
- Keep auth UX tree-structured under `auth/*`: `set-access`, `whoami`, `resolve`, `clean`/`clear-access`.
- `auth whoami` is the canonical live auth probe and should call Zendesk with the stored credentials unless `--check=false` is requested.
- In `env_or_file`, the global JSON config is sectioned by organization key; each profile stores `email`, `api_token`, and `auth_type`.
- Start with Zendesk Support API v2 on `https://{subdomain}.zendesk.com/api/v2`.
- The current scaffold stores one API-token credential set per organization key; internally this still maps to the Zendesk subdomain suffix.
- Use the `agent-facing-api` pattern for the CLI:
  - `zendesk-mgmt q '...'` is the primary read interface for agents.
  - `zendesk-mgmt grep ...` is the scoped full-text search path for local references, fixtures, and cached artifacts.
  - Defer `m`/write mutations until the read contract is stable.
- Keep the query layer compact: field projection, presets, batching, and machine-stable output are required.
- For per-ticket investigation workspaces, materialize artifacts into project-local `.attachments/` rather than temp-root dumps.
- If the user asks to inspect logs, do not ingest the full log into context up front.
  Search `.attachments/` first with targeted patterns, then read only narrow slices around the hits.
  Prefer `rg` on both macOS and Windows when available; on Windows without `rg`, use PowerShell `Select-String` as the fallback search path.

## Build, Test, and Development Commands
Run from the repo root once the Go scaffold exists:

- `./setup.sh`: canonical macOS source-checkout install/update flow.
- `./setup.sh --install-only`: safe reinstall of binary, skill artifact, links, and install-state metadata.
- `.\setup.ps1`: canonical Windows source-checkout install/update flow.
- `go run ./cmd/zendesk-mgmt version`: print embedded build metadata for install verification.
- `go test ./...`: default verification path.
- `go test ./... -run TestName -v`: focused regression run.
- `go fmt ./... && go vet ./...`: formatting and basic static checks.
- `go run ./cmd/zendesk-mgmt --help`: local CLI smoke test.
- `task-board q --format compact 'summary()'`: inspect board status before substantive work.

## Testing Guidelines
- Follow the `go-testing-tools` approach: keep tests closed-loop, fast, and table-driven.
- Prefer pure parser/query tests first, HTTP client tests second, workflow tests third.
- Use `httptest.Server` for Zendesk API simulations instead of live network calls.
- Add golden tests for compact output and schema/query rendering where output stability matters.
- If a TUI appears later, use `tuitestkit`; until then, keep the current CLI test harness standard-library-first.

## Task Board Workflow
- All board reads and writes must go through `task-board`.
- Before starting substantive work, locate the active item with `task-board q --format compact 'summary()'` and `task-board q --format compact 'get(ID) { full }'`.
- Keep findings in the board with `set_notes`, `add_checklist_item`, and resources when the output is reusable.
- When bootstrapping local agent runtime in this repo, use `agents-infra setup local "$(pwd)"`.

## Security & Configuration
- Store credentials in macOS Keychain under a repo-specific service name, not in committed files.
- For Windows-friendly setup, prefer the standard global config path exposed by `zendesk-mgmt auth config-path` over ad hoc repo-local files.
- Never commit Zendesk subdomain credentials, exported ticket payloads with secrets, or ad hoc debug dumps.
- Sanitize ticket bodies, emails, phone numbers, and attachments before turning live payloads into fixtures.

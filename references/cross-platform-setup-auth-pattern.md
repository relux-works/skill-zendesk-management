# Cross-Platform Setup And Credential Pattern

This document captures the reusable setup and credential pattern from `zendesk-mgmt`
so the same shape can be applied in other repo-local CLIs and skills.

## Goal

Standardize two things across projects:

- cross-platform local setup from a source checkout
- predictable, secure credential storage with the same user-facing auth commands

## First-Class Platform Matrix

Use this as the default matrix unless a project has a narrower scope:

- `macOS arm64`
- `macOS x86_64`
- `Windows x86_64`
- `Windows arm64`

Release artifacts should be produced for:

- `darwin/amd64`
- `darwin/arm64`
- `windows/amd64`
- `windows/arm64`

Linux is optional. Do not claim first-class Linux setup until both the installer path
and the credential storage story are explicitly verified.

## Credential Model

User-facing input should stay small:

- `organization`
- `email`
- `token`

Resolved internal shape should be stable:

- `organization`
- `instance_url` or another stable account key when needed
- `email`
- `api_token` or equivalent secret
- `auth_type`

Supported sources:

- `auto`
- `keychain`
- `env_or_file`

Keep the source names stable across tools. That allows internal storage changes
without forcing the user to relearn the auth surface.

## Default Source Policy

Recommended default `auto` mapping for new repos:

- `darwin` -> `keychain`
- `windows` -> `keychain`
- everything else -> `env_or_file` until a system keyring path is proven

Interpret `keychain` generically:

- on macOS, this means Keychain
- on Windows, this means Windows Credential Manager or another OS-native secret store

Use `env_or_file` as the explicit fallback for:

- CI
- ephemeral local overrides
- platforms where system keyring support is not yet validated

## Credential Storage Contract

### `keychain`

Use for the primary local-user path on supported desktop platforms.

- service name: binary or app identifier, for example `zendesk-mgmt`
- account key: a stable derived identifier, for example `https://acme.zendesk.com`
- stored payload: serialized credential object, not just the raw token

The stored payload should include enough metadata to avoid reconstructing user input
later, for example:

- `email`
- `api_token`
- `auth_type`

### `env_or_file`

Resolution order should be:

1. environment variables
2. global config file under `os.UserConfigDir()`

For this Zendesk pattern, the environment layer is:

1. `ZENDESK_EMAIL` + `ZENDESK_API_TOKEN`
2. `ZENDESK_EMAIL` + `ZENDESK_ACCESS_TOKEN` as a deprecated alias

The global config path should be:

- `os.UserConfigDir()/APP/auth.json`

Examples:

- macOS: `~/Library/Application Support/<app>/auth.json`
- Windows: `%AppData%\\<app>\\auth.json`

Keep the JSON file organization-scoped:

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

Never use repo-local auth files for normal operation.

## Auth Command Surface

Keep the auth tree structurally identical across tools:

- `auth set-access`
- `auth whoami`
- `auth resolve`
- `auth clean` or `auth clear-access`
- `auth config-path`

Recommended behavior:

- `set-access` stores credentials into the default source for the current platform
- `whoami` inspects local storage and can perform a live auth probe
- `whoami --check=false` skips the live probe
- `resolve` reports where the tool would load credentials from without printing the secret
- `clean` removes the stored credentials for the selected scope
- `config-path` prints the global auth-file location for the current machine

If a low-level `write-config` helper exists, keep it clearly documented as support-only,
not as the main UX.

## Setup Pattern

The setup contract should be identical on macOS and Windows, with platform-specific
tooling only where necessary.

### Entry Points

Provide thin root wrappers:

- `./setup.sh`
- `./setup.ps1`

Those wrappers should delegate to:

- `scripts/setup.sh`
- `scripts/setup.ps1`

### Responsibilities

Each setup script should do the same logical work:

1. resolve repo root
2. ensure Go is available
3. compute build metadata from git when available
4. build the CLI with embedded version metadata
5. install the binary into a user-local bin directory
6. copy a degitized installed skill artifact into `~/.agents/skills/<skill-name>`
7. refresh `~/.claude/skills/<skill-name>` and `~/.codex/skills/<skill-name>`
8. write install-state metadata into the app config directory
9. ensure the user-local bin dir is on `PATH`
10. verify the installed binary with no-side-effect commands

Platform-specific toolchain bootstrap:

- macOS: prefer `brew install go`
- Windows: prefer `winget install GoLang.Go`

## Install-State Contract

Store install metadata in:

- `os.UserConfigDir()/APP/install.json`

The payload should contain at least:

- `repoPath`
- `installedSkillPath`
- `binDir`
- `platform`
- `arch`
- `version`
- `commit`
- `buildDate`
- `installOnly`

This file is not secret material. It exists to support refresh, reinstall, and support flows.

## Verification Contract

A project using this pattern should be able to verify setup with:

```bash
go test ./...
<tool> version
<tool> auth config-path
```

Credential smoke flow:

1. `<tool> auth set-access ...`
2. `<tool> auth whoami --check=false ...`
3. `<tool> auth resolve ...`
4. `<tool> auth clean ...`

When live network checks exist, keep them in `whoami` and make them opt-out with
`--check=false`.

## Security Rules

- Prefer system secret storage on supported desktop platforms.
- Treat environment variables as override or CI input, not the primary desktop UX.
- Keep file fallback under `os.UserConfigDir()`, never inside the repo.
- Never commit tokens, exported secret dumps, or ad hoc debug files.
- Keep the auth command surface stable even if the storage backend changes.

## Copy-Forward Checklist For Other Repos

- rename the binary, service name, and config directory
- keep `auto`, `keychain`, and `env_or_file`
- keep the `auth/*` command tree
- keep install-state metadata
- keep source-checkout setup wrappers at repo root
- keep release artifacts for the declared platform matrix
- add tests for platform-default source selection and auth resolution order

## Current `zendesk-management` Status

This repo already matches most of the pattern:

- macOS setup path is implemented
- Windows setup path is implemented
- release matrix covers macOS and Windows on `amd64` and `arm64`
- auth command tree is stable
- `env_or_file` resolution order is implemented and tested

Current delta against the recommended target:

- `zendesk-management` still maps Windows `auto` to `env_or_file`
- that means Windows default writes to `%AppData%\\zendesk-mgmt\\auth.json`
- the recommended target for new repos is Windows `auto` -> system keyring

That delta should be treated as an implementation backlog item, not as the desired
long-term standard.

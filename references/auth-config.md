# Auth Config

Current token-loading model for `zendesk-mgmt`:

- `keychain`
- `env_or_file`

Credential shape:

- `organization`
- `email`
- `api_token`
- `auth_type=api_token`

## Source Selection

Default source by platform:

- `darwin` -> `keychain`
- everything else -> `env_or_file`

The CLI can also be forced explicitly:

```bash
zendesk-mgmt auth resolve --source keychain --organization acme
zendesk-mgmt auth resolve --source env_or_file --organization acme
```

## User-Facing Setup Command

The intended zero-thinking setup command is:

```bash
zendesk-mgmt auth set-access --organization acme --email agent@acme.com --token YOUR_TOKEN
```

What it does:

- on macOS, stores the email + API token payload in Keychain
- on Windows-friendly flows, stores the email + API token payload in the standard global config file

The user-facing auth tree is now:

```bash
zendesk-mgmt auth set-access --organization acme --email agent@acme.com --token YOUR_TOKEN
zendesk-mgmt auth whoami --organization acme
zendesk-mgmt auth whoami --organization acme --check=false
zendesk-mgmt auth resolve --organization acme
zendesk-mgmt auth clean --organization acme
```

`auth whoami` now does two things:

- inspects the local storage state
- performs a live Zendesk auth check against `GET /api/v2/users/me.json` by default

Use `--check=false` if you only want the local-storage part.

`write-config` still exists, but it is a low-level support command rather than the main UX.

The user only provides:

- organization, for example `acme`
- email
- token

The tool derives the instance URL as `https://acme.zendesk.com` when needed.

## Keychain Mode

Used primarily for macOS.

- service name: `zendesk-mgmt`
- account key: derived Zendesk instance URL, for example `https://acme.zendesk.com`

Current code supports both writing and reading for this mode through the suffix-derived instance URL.

## Env Or File Mode

Resolution order:

1. `ZENDESK_EMAIL` + `ZENDESK_API_TOKEN`
2. `ZENDESK_EMAIL` + `ZENDESK_ACCESS_TOKEN` as a deprecated alias
2. global JSON config file

That makes Windows-friendly setup simple without requiring keychain integration first.

## Global Config Path

The tool uses Go's standard `os.UserConfigDir()` and stores:

- `zendesk-mgmt/auth.json`

Examples:

- macOS: `~/Library/Application Support/zendesk-mgmt/auth.json`
- Windows: `%AppData%\\zendesk-mgmt\\auth.json`

Use the CLI to print the real path on the current machine:

```bash
zendesk-mgmt auth config-path
```

## JSON Schema

The global file is sectioned by organization. Each profile currently has:

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

The tool writes this file automatically via:

```bash
zendesk-mgmt auth set-access --organization acme --email agent@acme.com --token YOUR_TOKEN --source env_or_file
```

## Debugging

Inspect where the token would resolve from without printing the secret itself:

```bash
zendesk-mgmt auth whoami --source env_or_file --organization acme
zendesk-mgmt auth resolve --source env_or_file --organization acme
zendesk-mgmt auth whoami --source keychain --organization acme
zendesk-mgmt auth resolve --source keychain --organization acme
zendesk-mgmt auth clean --source env_or_file --organization acme
```

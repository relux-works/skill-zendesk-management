# Auth Config

Current token-loading model for `zendesk-mgmt`:

- `keychain`
- `env_or_file`

## Source Selection

Default source by platform:

- `darwin` -> `keychain`
- everything else -> `env_or_file`

The CLI can also be forced explicitly:

```bash
zendesk-mgmt auth resolve --source keychain --instance https://acme.zendesk.com
zendesk-mgmt auth resolve --source env_or_file
```

## User-Facing Setup Command

The intended zero-thinking setup command is:

```bash
zendesk-mgmt auth set-access --suffix acme --token YOUR_TOKEN
```

What it does:

- on macOS, stores the token in Keychain
- on Windows-friendly flows, stores the token in the standard global config file

The user-facing auth tree is now:

```bash
zendesk-mgmt auth set-access --suffix acme --token YOUR_TOKEN
zendesk-mgmt auth whoami --suffix acme
zendesk-mgmt auth resolve --suffix acme
zendesk-mgmt auth clear-access --suffix acme
```

`write-config` still exists, but it is a low-level support command rather than the main UX.

The user only provides:

- org suffix, for example `acme`
- token

The tool derives the instance URL as `https://acme.zendesk.com` when needed.

## Keychain Mode

Used primarily for macOS.

- service name: `zendesk-mgmt`
- account key: derived Zendesk instance URL, for example `https://acme.zendesk.com`

Current code supports both writing and reading for this mode through the suffix-derived instance URL.

## Env Or File Mode

Resolution order:

1. `ZENDESK_ACCESS_TOKEN`
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

The global file is sectioned by org suffix. Each profile currently has only one field:

```json
{
  "profiles": {
    "acme": {
      "access_token": "your-token-here"
    }
  }
}
```

The tool writes this file automatically via:

```bash
zendesk-mgmt auth set-access --suffix acme --token YOUR_TOKEN --source env_or_file
```

## Debugging

Inspect where the token would resolve from without printing the secret itself:

```bash
zendesk-mgmt auth whoami --source env_or_file --suffix acme
zendesk-mgmt auth resolve --source env_or_file --suffix acme
zendesk-mgmt auth whoami --source keychain --suffix acme
zendesk-mgmt auth resolve --source keychain --suffix acme
zendesk-mgmt auth clear-access --source env_or_file --suffix acme
```

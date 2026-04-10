# Implement Auth Config Sources

## Description
Implement access-token loading for zendesk-mgmt with explicit keychain and env or global-config modes.

## Scope
Add a Go module scaffold, config package, standard global config path resolution, keychain-backed loading on macOS, env-plus-global-config loading for Windows-friendly flows, and tests.

## Acceptance Criteria
Code resolves access tokens from keychain or env/global-config, the global JSON schema currently contains only access_token, Windows uses the standard user config dir, and tests cover path resolution and loading precedence.

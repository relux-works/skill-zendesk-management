## Status
done

## Assigned To
codex

## Created
2026-04-10T07:42:25Z

## Last Update
2026-04-10T07:52:21Z

## Blocked By
- (none)

## Blocks
- (none)

## Checklist
- [x] Bootstrap minimal Go module and auth config package
- [x] Support keychain mode for macOS token lookup
- [x] Support env and standard global JSON config path for Windows-friendly flow
- [x] Cover resolution and precedence with table-driven tests
- [x] Add user-facing set-access command that chooses platform-appropriate storage automatically

## Notes
User requested this explicitly through the board. Config file schema should currently stay minimal with one field: access_token.
Implemented a minimal Go scaffold with internal/config resolver and zendesk-mgmt auth commands. Token sources: keychain and env_or_file. env_or_file resolves ZENDESK_ACCESS_TOKEN first, then os.UserConfigDir()/zendesk-mgmt/auth.json. The JSON schema currently contains only access_token. Verified with go test ./... and CLI smoke commands for config-path and env_or_file resolve.
Follow-up: add a simple set-access command so the user passes org suffix and token once. On macOS store into keychain; on Windows-friendly flows persist into the global auth config automatically rather than relying on transient shell env state.
Implemented a zero-thinking user-facing bootstrap command: zendesk-mgmt auth set-access --suffix ORG --token TOKEN. On macOS it stores the token in Keychain under the derived instance URL https://ORG.zendesk.com. On Windows-friendly flows it stores the token in the standard global config file, sectioned by org suffix under profiles. resolve now accepts --suffix and can read the stored profile.

## Precondition Resources
(none)

## Outcome Resources
- [auth-config.md](file://TASK-260410-2v6pgg/auth-config.md) — Implemented auth config and token-source contract
- [mvp-contract.md](file://TASK-260410-2v6pgg/mvp-contract.md) — Updated MVP contract with token-source implementation

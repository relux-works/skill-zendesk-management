## Status
done

## Assigned To
codex

## Created
2026-04-10T09:09:37Z

## Last Update
2026-04-10T09:29:22Z

## Blocked By
- (none)

## Blocks
- (none)

## Checklist
(empty)

## Notes
Upgraded auth storage from token-only to organization + email + api_token with auth_type=api_token. set-access now requires --organization, --email, and --token; whoami/resolve expose email and auth_type without printing the secret; auth clean/cleanup/clear alias clear-access; keychain secrets are stored as JSON payloads while legacy plain-token secrets still inspect safely. Verification: go test ./..., go run ./cmd/zendesk-mgmt --help, sequential env_or_file smoke for set-access/whoami/resolve/clean, and installed CLI refresh via ./setup.sh --install-only.
Extended auth whoami with a live Zendesk probe that resolves credentials through the CLI storage layer and calls GET /api/v2/users/me.json using Basic auth for api_token credentials. Added internal/zendesk auth-check client + httptest coverage, default live_check output in whoami, and --check=false for local-only inspection. Verified with go test ./..., go run ./cmd/zendesk-mgmt auth whoami --organization revizto, and installed CLI refresh via ./setup.sh --install-only; live_check now returns HTTP 200 for revizto.

## Precondition Resources
(none)

## Outcome Resources
(none)

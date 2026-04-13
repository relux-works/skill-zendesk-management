## Status
done

## Assigned To
codex

## Created
2026-04-10T09:09:37Z

## Last Update
2026-04-10T11:15:22Z

## Blocked By
- (none)

## Blocks
- (none)

## Checklist
(empty)

## Notes
Extended auth whoami with a live Zendesk probe that resolves credentials through the CLI storage layer and calls GET /api/v2/users/me.json using Basic auth for api_token credentials. Added internal/zendesk auth-check client + httptest coverage, default live_check output in whoami, and --check=false for local-only inspection. Verified with go test ./..., go run ./cmd/zendesk-mgmt auth whoami --organization <org>, and installed CLI refresh via ./setup.sh --install-only; live_check now returns HTTP 200 for a configured org.

## Precondition Resources
(none)

## Outcome Resources
(none)

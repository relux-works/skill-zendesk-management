## Status
done

## Assigned To
codex

## Created
2026-04-10T08:32:43Z

## Last Update
2026-04-10T08:34:41Z

## Blocked By
- (none)

## Blocks
- (none)

## Checklist
(empty)

## Notes
Updated the user-facing auth naming from suffix to organization across CLI usage, flag descriptions, examples, and JSON output fields. Internal config semantics remain suffix-based for instance derivation, and the deprecated --suffix flag is still accepted as a compatibility alias. Verification: go test ./..., go run ./cmd/zendesk-mgmt --help, sequential auth smoke with set-access/whoami/resolve/clear via --organization, and compatibility smoke with whoami --suffix.

## Precondition Resources
(none)

## Outcome Resources
(none)

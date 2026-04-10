## Status
done

## Assigned To
codex

## Created
2026-04-10T08:01:40Z

## Last Update
2026-04-10T08:05:38Z

## Blocked By
- (none)

## Blocks
- (none)

## Checklist
- [x] Add auth whoami command for platform-appropriate inspection
- [x] Add auth clear-access command for keychain and global config cleanup
- [x] Keep user-facing auth tree compact and treat write-config as low-level support only
- [x] Cover cross-platform auth tree behavior with tests and smoke commands

## Notes
User explicitly wants tree-structured commands. This task keeps the auth UX under auth/* and must not regress macOS vs Windows-friendly storage behavior.
Implemented auth whoami and auth clear-access under the existing auth tree. The user-facing auth family is now set-access, whoami, resolve, and clear-access, with write-config left as a low-level support path. clear-access removes the derived keychain entry on macOS or the suffix-scoped global config profile on Windows-friendly flows. Verified with go test ./... and a sequential smoke flow: set-access -> whoami -> resolve -> clear-access in env_or_file mode.

## Precondition Resources
(none)

## Outcome Resources
- [auth-config.md](file://TASK-260410-3iudm5/auth-config.md) — Auth command tree and storage behavior

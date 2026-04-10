## Status
done

## Assigned To
codex

## Created
2026-04-09T21:51:21Z

## Last Update
2026-04-10T08:27:00Z

## Blocked By
- (none)

## Blocks
- (none)

## Checklist
- [x] Describe the canonical source-checkout installer pattern after skill-project-management
- [x] Require installed skill copy under ~/.agents/skills and refreshed Claude/Codex links
- [x] Require install-state metadata and a safe reinstall or install-only flow
- [x] Require automatic setup support for macOS arm64 macOS x86_64 and Windows x86_64
- [x] Require automatic setup support for macOS arm64 macOS x86_64 Windows x86_64 and Windows arm64

## Notes
Reference pattern: skill-project-management uses a single source-checkout setup entrypoint, installs binaries into ~/.local/bin, copies a degitized installed skill artifact, refreshes Claude/Codex links, writes install state, and supports release artifacts via goreleaser. This task must adapt that pattern for zendesk-mgmt and the Zendesk skill.
Implementation started. Deliverable now includes scripts/setup.sh, scripts/setup.ps1, root wrappers, skill artifact installation, install-state metadata, and a release matrix that covers both Intel and ARM on macOS and Windows.
Implemented source-checkout setup for zendesk-mgmt and the installed zendesk-management skill: scripts/setup.sh, scripts/setup.ps1, root wrappers, installed skill copy under ~/.agents/skills/zendesk-management, refreshed ~/.claude and ~/.codex links, install-state metadata, and a goreleaser build matrix for darwin/windows amd64/arm64. Added embedded version metadata and install verification via zendesk-mgmt version plus zendesk-mgmt auth config-path. Verification completed on macOS arm64 with ./setup.sh --install-only, go test ./..., goreleaser check, and goreleaser build --snapshot --clean producing darwin/amd64, darwin/arm64, windows/amd64, and windows/arm64 artifacts. setup.ps1 was validated by code review only on this machine because pwsh is not installed.

## Precondition Resources
- [mvp-contract.md](file://TASK-260410-1qwfad/mvp-contract.md) — Setup section in the MVP contract

## Outcome Resources
(none)

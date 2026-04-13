# STORY-260414-2ns835: ticket-analysis-workspace-flow

## Description
Build the operator workflow for per-ticket investigation workspaces. In a project folder dedicated to one ticket, the CLI should materialize ticket artifacts into a predictable local directory, expand archives, sanitize text before agent-facing consumption, and guide log reading through grep-first search instead of full-file dumps.

## Scope
- Add a ticket-level export/materialize command that downloads all ticket attachments into a project-local workspace directory.
- Default the workspace artifact root to .attachments/ under the current project directory so agents do not depend on temp/cwd quirks.
- Expand supported archives into stable subdirectories.
- Postprocess extracted and standalone text-like files with anonymization before exposing them to the agent.
- Emit a machine-stable manifest so later tooling can find processed files.
- Document grep-first log investigation flow with cross-platform guidance for macOS and Windows.

## Acceptance Criteria
- a ticket-level command materializes attachments into a project-local .attachments/ directory.
- archive attachments are expanded into readable subtrees.
- text-like files are anonymized before the agent-facing path is returned.
- non-text binaries remain available as files without naive full-text expansion.
- the command writes a manifest describing processed outputs.
- skill/docs instruct agents to grep logs first, then read targeted slices, with a Windows-compatible search path.

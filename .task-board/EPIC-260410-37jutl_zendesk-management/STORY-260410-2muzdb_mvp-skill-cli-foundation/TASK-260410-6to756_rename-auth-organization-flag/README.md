# Rename Auth Organization Flag

## Description
Rename the user-facing auth organization flag from suffix to organization across CLI and local docs.

## Scope
Keep internal suffix semantics in config, but rename the user-facing auth flag and docs to organization.

## Acceptance Criteria
CLI help, examples, and auth JSON output use organization for the user-facing field while preserving compatibility with existing suffix behavior.

# Switch Windows Auto Auth To System Keyring

## Description
Change the default Windows auto auth path from env_or_file-backed AppData JSON to the OS system credential store while keeping env_or_file as an explicit fallback for CI and manual overrides.

## Scope
Update auth source selection for Windows, adjust set-access/whoami/resolve/clean semantics, refresh docs, and cover the new default with tests.

## Acceptance Criteria
Windows auto storage uses the system secret store by default, explicit env_or_file still works, docs no longer claim Windows defaults to AppData JSON, and tests cover set-access plus credential resolution on Windows.

# Polish Auth Command Tree

## Description
Clean up the auth command family so the user-facing credential flow stays under auth and works consistently across macOS and Windows-friendly paths.

## Scope
Keep tree-structured auth commands, add whoami and clear-access, preserve set-access, and ensure platform-appropriate storage behavior for keychain and global config profiles.

## Acceptance Criteria
auth provides a coherent tree of user-facing commands, clear-access removes the correct storage entry on each platform, whoami reports the effective storage path or account key without leaking the token, and tests cover the cross-platform behavior.

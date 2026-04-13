# Harden Windows Auth Storage

## Description
Review and improve the default Windows credential storage flow for zendesk-mgmt so setup stays cross-platform but secrets land in the right place.

## Scope
Validate current Windows x86_64 support, document the existing env_or_file behavior, and prepare a change to move the default Windows auto path toward a system secret store.

## Acceptance Criteria
Board contains a concrete task describing the Windows auth-storage improvement, the current behavior is captured in notes, and the follow-up can be picked up without redoing investigation.

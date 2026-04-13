## Status
done

## Assigned To
codex

## Created
2026-04-10T09:58:14Z

## Last Update
2026-04-10T11:15:22Z

## Blocked By
- (none)

## Blocks
- (none)

## Checklist
(empty)

## Notes
Implemented read-only attachment support: q attachment(id), q ticket_attachments(ticket_id=...), and attachment download ATTACHMENT_ID with host-aware auth handling for content_url downloads. Added attachment schema/presets/docs and tests for ticket attachment flattening plus same-host vs external-host download auth behavior. Live smoke passed for attachment-aware search, ticket_attachments(ticket_id=<ticket_id>), attachment(<attachment_id>), and attachment download to a local destination.

## Precondition Resources
(none)

## Outcome Resources
(none)

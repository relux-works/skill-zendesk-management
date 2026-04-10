## Status
done

## Assigned To
codex

## Created
2026-04-10T09:58:14Z

## Last Update
2026-04-10T10:02:52Z

## Blocked By
- (none)

## Blocks
- (none)

## Checklist
(empty)

## Notes
Implemented read-only attachment support: q attachment(id), q ticket_attachments(ticket_id=...), and attachment download ATTACHMENT_ID with host-aware auth handling for content_url downloads. Added attachment schema/presets/docs and tests for ticket attachment flattening plus same-host vs external-host download auth behavior. Live smoke on revizto passed for search(query="type:ticket has_attachment:true"), ticket_attachments(ticket_id=68159), attachment(15759358677263), and attachment download 15759358677263.

## Precondition Resources
(none)

## Outcome Resources
(none)

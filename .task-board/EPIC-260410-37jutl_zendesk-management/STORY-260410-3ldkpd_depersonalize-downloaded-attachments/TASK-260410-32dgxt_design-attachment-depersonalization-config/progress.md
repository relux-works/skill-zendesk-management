## Status
backlog

## Assigned To
(none)

## Created
2026-04-10T12:54:32Z

## Last Update
2026-04-10T12:59:31Z

## Blocked By
- (none)

## Blocks
- (none)

## Checklist
(empty)

## Notes
Examples to preserve in design:
- Google Drive:/Clients/SecretCo/Invoices/report.pdf -> Google Drive:/<seg:ab12cd>/<seg:98ef01>/report.pdf
- https://server01.internal.example/share/folder1/file.txt -> https://<host:8f21ac>/<seg:4a9d22>/file.txt
- password=supersecret -> password=<secret:0b19fa>
- Authorization: Bearer eyJ... -> Authorization: Bearer <token:91c2de>
- Deterministic redaction should optionally preserve a small prefix and suffix around the redacted token when configured, for example abc<secret:91c2de>xyz, so operators keep rough orientation without seeing the full original.
- Prefix/suffix preservation must be configurable per rule, and high-risk secret classes must be allowed to set both to 0.
- The share of the original that remains visible versus replaced should be rule-configurable via a 0..1 fraction.
- Visibility mode should be rule-configurable too: head, tail, middle, or head_tail.
- Exact char preservation and percentage-based preservation should be able to coexist, with implementation rules documented for precedence.
Repeated original values should map to the same token because the redaction uses a local secret salt/HMAC, not a plain unsalted hash.

## Precondition Resources
(none)

## Outcome Resources
(none)

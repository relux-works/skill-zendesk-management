# TASK-260410-32dgxt: design-attachment-depersonalization-config

## Description
Design the config schema and processing contract for attachment depersonalization. Define how attachment download selects postprocessors, how the text engine matches eligible files, and how regex-group masking plus built-in sensitive-value transforms are expressed in config.

## Scope
- Define a separate global config file/section for non-secret tool behavior such as attachment sanitization.
- Define an engine registry shape so multiple postprocessors can exist later, while only the text engine is enabled now.
- Define shared hashing settings (salt/HMAC strategy, token length, labels).
- Define text-engine matching rules by extension and/or MIME type.
- Define transform rules for paths, hosts, IPs, logins, passwords, tokens, keys, and arbitrary regex capture groups.
- Define preservation knobs for provider labels and path tail segments/filenames.
- Define per-rule visibility controls including visible-layout mode (head, tail, middle, head_tail) and configurable visible_fraction/mask_fraction in the range 0..1.
- Define how visibility mode interacts with preserve_prefix_chars and preserve_suffix_chars for cases where exact char counts are preferred over percentages.
- Define failure behavior and whether raw originals are discarded, quarantined, or allowed only under explicit override.

## Acceptance Criteria
- config example shows multiple postprocessors with only text enabled.
- config example shows regex capture-group masking for text docs.
- config example shows stable HMAC redaction for path segments, hosts, IPs, and secrets.
- config example covers preserving Google Drive label while hashing sensitive path details.
- config example covers optional tail preservation for the last path segment or filename.
- config example defines configurable visibility controls, including mode=head|tail|middle|head_tail and a 0..1 visibility/masking fraction.
- processing contract states how sanitized output reaches destination and what happens to the raw original.

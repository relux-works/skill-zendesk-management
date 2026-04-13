# STORY-260410-3ldkpd: depersonalize-downloaded-attachments

## Description
Add a pluggable post-download attachment postprocessing pipeline so downloaded Zendesk attachments can be sanitized before any agent-facing consumption. The first engine targets text-like documents and depersonalizes sensitive values while preserving enough operational context for debugging.

## Scope
- Extend attachment download flow to run postprocessors after fetch and before the final artifact is handed to the user/agent.
- Keep processor configuration in a non-secret global config file under the zendesk-mgmt config directory, separate from auth secrets.
- Support multiple postprocessors in config, with only the text-document engine required in the first slice.
- Text postprocessing should support regex-based masking/removal groups plus built-in handling for paths, hostnames/server names, IP addresses, logins, passwords, tokens, and keys.
- Sensitive values should be replaced with deterministic salted HMAC-based tokens so repeated occurrences stay linkable without revealing originals.
- Safe high-level context may be preserved when configured, for example keeping a provider label like Google Drive and optionally the last path segment or filename tail while hashing internal details.

## Acceptance Criteria
- attachment download has a postprocessing pipeline concept and a config shape for multiple engines.
- the first implemented engine targets text-like files only.
- config supports regex/capture-group based redaction rules for text documents.
- paths, hosts, IPs, logins, passwords, tokens, and keys can be deterministically redacted with salted stable tokens.
- repeated originals map to the same redacted token within the local config scope.
- provider labels like Google Drive can be preserved while sensitive path internals are hashed, with optional tail preservation.
- when sanitization is enabled, raw attachments are not exposed to the agent path unless an explicit bypass/debug policy says otherwise.

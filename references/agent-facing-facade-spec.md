# Agent-Facing Zendesk Facade Spec

## Goal

Implement a read-only, agent-optimized Zendesk facade in `zendesk-mgmt`
using the `agent-facing-api` pattern:

- `q` for structured reads through a compact DSL
- `grep` for low-friction text search
- `attachment download` for binary attachment retrieval
- `auth` remains the credential/bootstrap surface

This MVP is for Support/Ticketing data only and is explicitly optimized for
AI-agent consumption, low token overhead, and predictable transport behavior.

## Non-goals

- write/mutation commands
- attachment uploads or attachment mutations
- audits, incremental exports, or admin configuration APIs
- browser-side auth flows
- local cache/indexing layer

## Official Sources

- Security and authentication:
  https://developer.zendesk.com/api-reference/introduction/security-and-auth/
- Pagination:
  https://developer.zendesk.com/api-reference/introduction/pagination/
- Rate limits:
  https://developer.zendesk.com/api-reference/introduction/rate-limits/
- Tickets:
  https://developer.zendesk.com/api-reference/ticketing/tickets/tickets/
- Ticket comments:
  https://developer.zendesk.com/api-reference/ticketing/tickets/ticket_comments/
- Users:
  https://developer.zendesk.com/api-reference/ticketing/users/users/
- Organizations:
  https://developer.zendesk.com/api-reference/ticketing/organizations/organizations/
- Organization memberships:
  https://developer.zendesk.com/api-reference/ticketing/organizations/organization_memberships/
- Search:
  https://developer.zendesk.com/api-reference/ticketing/ticket-management/search/
- Attachments:
  https://developer.zendesk.com/api-reference/ticketing/tickets/ticket-attachments/

## Auth And Transport

- Runtime entrypoints:
  - `zendesk-mgmt q '<query>' --organization <org> --format json|compact`
  - `zendesk-mgmt grep '<text>' --organization <org> --type ticket|user|organization --format json|compact`
  - `zendesk-mgmt attachment download <attachment_id> --organization <org> [--destination PATH]`
- `--organization` resolves stored credentials and instance URL via the
  existing local auth store.
- Current auth mode is `api_token` only:
  - username: `{email}/token`
  - password: `{api_token}`
  - header: `Authorization: Basic ...`
- Future `oauth_bearer` support is reserved but out of MVP.
- `q` and `grep` are read-only. Any future write surface must be a separate
  `m` command family.

## Entity Scope

MVP entities:

- tickets
- ticket comments
- attachments
- users
- organizations
- organization memberships
- search results

Why this set:

- tickets/comments/attachments cover the primary support workflow
- users/organizations/memberships provide the minimum join surface around
  requester and account context
- search gives a unified discovery surface across tickets, users, and
  organizations

## CLI Surface

### `q`

Single entrypoint for structured reads.

Syntax:

```text
<operation>(<params>) { <fields-or-preset> }
```

Rules:

- semicolon batches multiple queries in one CLI call
- field projection is always supported
- named presets are supported per entity
- `schema()` is mandatory for discovery
- output is transport-selected with `--format json|compact`

Examples:

```bash
zendesk-mgmt q 'schema()' --organization revizto --format json
zendesk-mgmt q 'ticket(12345) { overview }' --organization revizto --format compact
zendesk-mgmt q 'ticket_comments(ticket_id=12345, limit=5) { default }' --organization revizto --format compact
zendesk-mgmt q 'search(query="type:ticket status:open") { overview }' --organization revizto --format compact
zendesk-mgmt q 'ticket(12345) { overview }; user(67890) { minimal }' --organization revizto --format compact
```

### `grep`

Remote, scoped text search convenience layer for agents.

It is not local `rg`; it is a thin wrapper over Zendesk Search API tuned for
quick discovery and compact output.

Examples:

```bash
zendesk-mgmt grep 'invoice failed' --organization revizto --type ticket --format compact
zendesk-mgmt grep 'Acme' --organization revizto --type organization --format json
```

Rules:

- default backend is `GET /api/v2/search`
- grep is for discovery, not for deep scans
- grep inherits Search API limits, indexing lag, and offset-only pagination

## Query Operations

### Required Operations

`schema()`

- returns operations, fields, presets, filterable params, transport modes,
  and pagination capabilities

`ticket(id)`

- endpoint: `GET /api/v2/tickets/{ticket_id}`
- use for exact ticket fetch
- when `attachments` is selected by preset or explicit field, the facade also
  walks ticket comments and returns compact attachment refs (`id` +
  `file_name`)

`tickets(after=?, limit=?, page=?, per_page=?)`

- endpoint: `GET /api/v2/tickets`
- cursor-first transport
- offset supported only as compatibility path
- no rich filtering in MVP; agents should use `search()` for ticket filtering

`ticket_comments(ticket_id, after=?, limit=?, page=?, per_page=?)`

- endpoint: `GET /api/v2/tickets/{ticket_id}/comments`
- cursor-first transport

`attachment(id)`

- endpoint: `GET /api/v2/attachments/{attachment_id}`
- returns attachment metadata

`ticket_attachments(ticket_id, after=?, limit=?, page=?, per_page=?)`

- transport endpoint: `GET /api/v2/tickets/{ticket_id}/comments`
- facade flattens comment attachment arrays into attachment rows with comment
  context

`user(id)`

- endpoint: `GET /api/v2/users/{user_id}`

`users(after=?, limit=?, page=?, per_page=?, role=?)`

- endpoint: `GET /api/v2/users`
- cursor-first transport
- `role` is the only MVP filter exposed directly on the list operation

`organization(id)`

- endpoint: `GET /api/v2/organizations/{organization_id}`

`organizations(after=?, limit=?, page=?, per_page=?)`

- endpoint: `GET /api/v2/organizations`
- cursor-first transport

`organization_memberships(organization_id=?, user_id=?, after=?, limit=?, page=?, per_page=?)`

- endpoints:
  - `GET /api/v2/organization_memberships`
  - `GET /api/v2/organizations/{organization_id}/organization_memberships`
  - `GET /api/v2/users/{user_id}/organization_memberships`
- facade picks the narrowest endpoint based on params

`search(query, include=?, sort_by=?, sort_order=?, page=?, per_page=?)`

- endpoint: `GET /api/v2/search?query=...`
- unified discovery over tickets, users, and organizations
- offset-only

`search_count(query)`

- endpoint: `GET /api/v2/search/count?query=...`
- returns count only

`search_export(type, query, after=?, limit=100)`

- endpoint: `GET /api/v2/search/export?...`
- for large result sets
- requires `type=ticket|organization|user|group`
- uses cursor pagination

## Projection And Presets

Every operation supports either explicit fields or a preset.

### Ticket

- `minimal`: `id subject status updated_at`
- `default`: `id subject status priority requester_id assignee_id updated_at attachments`
- `overview`: `id subject status priority type requester_id assignee_id organization_id updated_at tags attachments`
- `full`: normalized ticket payload suitable for direct agent reasoning

### Ticket Comment

- `minimal`: `id author_id public created_at body`
- `default`: `id author_id public created_at body html_body via`
- `overview`: `id author_id public created_at plain_body attachments via`
- `full`: normalized comment payload with metadata

### Attachment

- `minimal`: `id file_name size content_type`
- `default`: `id file_name size content_type content_url malware_scan_result`
- `overview`: `id file_name size content_type content_url ticket_id comment_id comment_author_id comment_public comment_created_at malware_scan_result`
- `full`: normalized attachment payload

### User

- `minimal`: `id name role organization_id`
- `default`: `id name email role organization_id active suspended updated_at`
- `overview`: `id name email role organization_id phone tags active suspended updated_at`
- `full`: normalized user payload with `user_fields`

### Organization

- `minimal`: `id name`
- `default`: `id name external_id shared_tickets shared_comments updated_at`
- `overview`: `id name external_id domain_names tags shared_tickets shared_comments updated_at`
- `full`: normalized organization payload with `organization_fields`

### Organization Membership

- `minimal`: `id user_id organization_id`
- `default`: `id user_id organization_id default view_tickets updated_at`
- `overview`: `id user_id organization_id organization_name default view_tickets updated_at`
- `full`: normalized membership payload

### Search Result

- `minimal`: `result_type id url`
- `default`: `result_type id url updated_at`
- `overview`: type-aware summary fields
- `full`: raw search object as returned by Zendesk for the result type

## Schema Contract

`schema()` must expose:

- operations
- per-operation parameter metadata
- per-entity fields
- presets
- default fields
- filterable params
- sortable params
- pagination mode per operation
- format modes: `json`, `compact`
- auth modes currently supported by runtime

Example shape:

```json
{
  "operations": ["schema", "ticket", "tickets", "ticket_comments", "attachment", "ticket_attachments", "user", "users", "organization", "organizations", "organization_memberships", "search", "search_count", "search_export"],
  "formats": ["json", "compact"],
  "pagination": {
    "tickets": ["cursor", "offset"],
    "ticket_comments": ["cursor", "offset"],
    "users": ["cursor", "offset"],
    "organizations": ["cursor", "offset"],
    "organization_memberships": ["cursor", "offset"],
    "search": ["offset"],
    "search_export": ["cursor"]
  }
}
```

## Pagination Policy

Zendesk recommends cursor pagination where available. The facade must make
cursor the default transport whenever an endpoint supports it.

Facade parameter normalization:

- `limit` maps to `page[size]` on cursor endpoints
- `after` maps to `page[after]` on cursor endpoints
- `per_page` and `page` are compatibility parameters for offset endpoints

Per operation:

- `tickets`, `ticket_comments`, `users`, `organizations`,
  `organization_memberships`: default to cursor pagination
- `ticket_attachments`: inherits pagination from `ticket_comments`
- `search`: offset only
- `search_export`: cursor only

Validation rules:

- most cursor-capable endpoints are capped at 100 records per page; clamp or
  reject larger `limit`
- `search()` must reject local requests that would page past the Zendesk
  10,000-record offset ceiling
- `search_export()` defaults to `limit=100` even though Zendesk permits up to
  1000, because the docs explicitly warn about slowdown/timeouts at 1000 and
  recommend 100
- `search_export()` must surface that `after_cursor` expires after one hour

## Search And Grep Semantics

### `search()`

- unified typed search
- supports `include`, `sort_by`, `sort_order`
- results are subject to search index lag of a few minutes
- returns up to 1000 results total, max 100 per page
- may return duplicate results when paging because it is offset-based

### `search_export()`

- use when expected results exceed 1000
- requires `filter[type]`; type in the raw query string is invalid for this
  endpoint
- ordered only by `created_at`
- no `count` in response

### `grep`

- optimized for quick discovery
- internally compiles to `search()`, not `search_export()`
- default columns in compact mode:
  - ticket: `id subject status priority requester_id updated_at`
  - user: `id name email role organization_id updated_at`
  - organization: `id name external_id updated_at`

## Output Contract

### `--format json`

- canonical JSON
- predictable for `jq` and tooling

### `--format compact`

- token-efficient text transport for agents
- list queries: CSV-style header + rows
- single entity queries: `key:value`
- batch queries: blank-line separated blocks with stable ordering

Errors are always structured:

```json
{
  "error": {
    "code": "rate_limited",
    "message": "Zendesk rate limit exceeded",
    "http_status": 429,
    "retryable": true,
    "details": {
      "retry_after_seconds": 41
    }
  }
}
```

## Error Contract

Stable error codes:

- `auth_invalid`
- `not_found`
- `invalid_query`
- `invalid_pagination`
- `rate_limited`
- `upstream_unavailable`
- `network_error`
- `unsupported_operation`

Rules:

- preserve Zendesk HTTP status when available
- include endpoint name and request id if Zendesk returns it
- include parsed rate-limit headers when relevant
- never print secrets

## Rate-Limit Policy

Global handling:

- parse `X-Rate-Limit` and `X-Rate-Limit-Remaining`
- parse ticketing headers such as `ratelimit-limit`,
  `ratelimit-remaining`, `ratelimit-reset`,
  `zendesk-ratelimit-tickets-index`
- on `429`, honor `Retry-After`

Search-specific handling:

- parse `Zendesk-RateLimit-search-index`
- treat search as having its own endpoint budget in addition to the global
  account budget

Retry policy:

- one automatic retry for idempotent reads after waiting `Retry-After`
- if the second attempt still fails with `429`, return structured error with
  retry metadata

## Behavioral Notes From Zendesk Docs

- search indexing can lag by a few minutes, so `search()` and `grep` are not
  strict freshness probes
- list tickets excludes archived tickets
- ticket comments are read from the Ticket Comments API, but comment creation
  belongs to the Tickets API and is out of scope for this MVP
- attachment metadata is read from the Attachments API; binary download uses
  the returned `content_url`, which may be externally hosted
- because `content_url` may be external, auth headers must not be blindly
  forwarded to arbitrary hosts
- ticket comment count is approximate; do not use it as a strict invariant
- `organization_id` on user payload is the user's default organization when
  multiple memberships exist; use organization memberships for the full join

## Package Layout

Suggested implementation layout:

```text
cmd/zendesk-mgmt/
internal/zendesk/
  client.go
  auth_transport.go
  rate_limit.go
  models.go
  fields.go
  query_ops.go
  grep.go
  pagination.go
  search.go
  tickets.go
  attachments.go
  users.go
  organizations.go
  organization_memberships.go
  compact_render.go
```

## Implementation Slices

### Slice 1: HTTP Client And Auth

- shared Zendesk client
- resolve creds from existing config layer
- Basic auth transport for `api_token`
- common response/error decoding

### Slice 2: Field Registry And Rendering

- entity field whitelist
- presets
- JSON renderer
- compact renderer

### Slice 3: Query DSL Skeleton

- parser
- `schema()`
- batching
- parameter normalization

### Slice 4: Core Entity Reads

- `ticket`
- `tickets`
- `ticket_comments`
- `attachment`
- `ticket_attachments`
- `user`
- `users`
- `organization`
- `organizations`
- `organization_memberships`

### Slice 5: Search Surface

- `search`
- `search_count`
- `search_export`
- `grep`

### Slice 6: Hardening

- rate-limit handling
- pagination validation
- stable errors
- live smoke coverage against a real account behind opt-in env gates

## Testing Strategy

Use `go-testing-tools` patterns throughout.

Required coverage:

- table-driven DSL parser tests
- `httptest` server coverage for every operation
- rate-limit tests for `429` + `Retry-After`
- snapshot/golden tests for `--format compact`
- schema snapshot tests
- auth transport tests for Basic auth header construction
- pagination mapping tests:
  - cursor requests
  - offset requests
  - deep-offset rejection for `search()`
- response normalization tests for each entity preset

Optional live smoke tests:

- gated behind explicit env opt-in
- `users/me`
- one known ticket lookup
- one `search()` query

## First Build Order

Recommended implementation order:

1. client + auth + error/rate limit plumbing
2. field registry + compact renderer
3. parser + `schema()`
4. `ticket`, `user`, `organization`
5. list endpoints with cursor transport
6. `search`, `search_count`, `grep`
7. `ticket_comments`
8. `organization_memberships`
9. `search_export`

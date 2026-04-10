# Zendesk API Notes

This project targets the Zendesk Support API (Ticketing API) on the standard account base URL:

- `https://{subdomain}.zendesk.com/api/v2`

## Authentication

Zendesk's official auth docs say API requests require a verified user and can be authorized either with:

- email address + API token
- OAuth access token

For this repo's first internal-use CLI, the simplest supported path is email + API token over Basic auth:

- credential format: `{email}/token:{api_token}`
- transport: `Authorization: Basic ...`

Practical local rule for this repo:

- store credentials in macOS Keychain only
- do not put tokens in git, `.env`, or project config
- keep the internal credential model flexible enough to add OAuth bearer auth later

Important auth note from the official docs:

- browser-side CORS requests must use OAuth access tokens
- API tokens are fine for server-side CLI usage, which is exactly this repo's starting case

## Initial Read Endpoints

These endpoints are enough for the first useful read-only CLI slice.

### Tickets

- `GET /api/v2/tickets`
- `GET /api/v2/tickets/{id}`

Notes:

- official docs explicitly distinguish Tickets API from Requests API
- for full-account ticket dumps, Zendesk recommends Incremental Ticket Export rather than relying on `List Tickets`

### Ticket Comments

- `GET /api/v2/tickets/{ticket_id}/comments`

Notes:

- comments are readable through the Ticket Comments API
- comment creation does **not** happen there; official docs state ticket comments are created through the Tickets API

### Search

- `GET /api/v2/search?query={query}`
- `GET /api/v2/search/export?query={query}&filter[type]={ticket|user|organization|group}`

Notes:

- search is the best first unified read surface for tickets, users, and organizations
- normal search is capped at 1,000 results per query and 100 per page
- normal `GET /search` uses offset pagination only
- search indexing can lag by a few minutes for newly created records
- export search is the large-result path and uses cursor pagination
- `GET /search` has its own endpoint-specific rate limit and returns `Zendesk-RateLimit-search-index` headers

### Users

- `GET /api/v2/users`
- `GET /api/v2/users/{id}`

Notes:

- users endpoint supports cursor pagination
- users responses include `organization_id`, which is enough for the first join into organizations

### Organizations

- `GET /api/v2/organizations`
- `GET /api/v2/organizations/{id}`
- `GET /api/v2/organization_memberships`

Notes:

- organizations support cursor pagination
- organization memberships are the clean follow-up path when we need to enumerate users inside organizations
- `organization_memberships` supports cursor pagination, recommends it, and returns at most 100 records per page

## Pagination

Zendesk recommends cursor pagination instead of offset pagination where possible.

Implications for this repo:

- prefer cursor pagination in users, organizations, and tickets list flows
- keep offset pagination support only where the API forces it, especially `GET /search`
- do not design the local DSL around page numbers only; cursor support must be a first-class transport concern

## Rate Limits

Zendesk's docs call out both account-wide limits and endpoint-specific limits.

Implementation requirements:

- handle `429 Too Many Requests`
- respect `Retry-After`
- surface rate-limit headers when available in debug output and tests
- avoid designing batch commands that accidentally turn into unbounded account scans
- parse account-wide headers such as `X-Rate-Limit` and `X-Rate-Limit-Remaining`
- support endpoint-specific headers where available, especially search-index headers

## Recommended First Live Smoke Calls

Once credentials are in Keychain, the first safe live reads should be:

1. `GET /api/v2/users/{id}` or another small known resource
2. `GET /api/v2/tickets/{id}` for a known ticket
3. `GET /api/v2/tickets/{ticket_id}/comments`
4. `GET /api/v2/search?query=type:ticket status<solved`

The exact `users/me`-style probe can be added later if we decide to standardize on a dedicated auth-check endpoint.

## Official Sources

- Security and authentication:
  - https://developer.zendesk.com/api-reference/introduction/security-and-auth/
- Ticketing API overview:
  - https://developer.zendesk.com/api-reference/ticketing/introduction/
- Tickets:
  - https://developer.zendesk.com/api-reference/ticketing/tickets/tickets/
- Ticket comments:
  - https://developer.zendesk.com/api-reference/ticketing/tickets/ticket_comments/
- Search:
  - https://developer.zendesk.com/api-reference/ticketing/ticket-management/search/
- Search guide:
  - https://developer.zendesk.com/documentation/ticketing/using-the-zendesk-api/searching-with-the-zendesk-api/
- Users:
  - https://developer.zendesk.com/api-reference/ticketing/users/users/
- Organizations:
  - https://developer.zendesk.com/api-reference/ticketing/organizations/organizations/
- Organization memberships:
  - https://developer.zendesk.com/api-reference/ticketing/organizations/organization_memberships/
- Pagination:
  - https://developer.zendesk.com/api-reference/introduction/pagination/
- Rate limits:
  - https://developer.zendesk.com/api-reference/introduction/rate-limits/
- Best practices for avoiding rate limiting:
  - https://developer.zendesk.com/documentation/ticketing/using-the-zendesk-api/best-practices-for-avoiding-rate-limiting

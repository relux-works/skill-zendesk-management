package zendesk

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/relux-works/skill-zendesk-management/internal/config"
)

const (
	defaultCursorLimit      = 25
	defaultOffsetPerPage    = 25
	defaultSearchExportSize = 100
	maxCursorPageSize       = 100
	maxOffsetPerPage        = 100
	maxSearchExportPageSize = 1000
)

type PageInfo struct {
	Mode         string `json:"mode,omitempty"`
	NextPage     string `json:"next_page,omitempty"`
	PreviousPage string `json:"previous_page,omitempty"`
	HasMore      bool   `json:"has_more,omitempty"`
	AfterCursor  string `json:"after_cursor,omitempty"`
	Count        *int   `json:"count,omitempty"`
}

type ListResponse struct {
	Items []map[string]any `json:"items"`
	Page  PageInfo         `json:"page,omitempty"`
}

type ListOptions struct {
	After   string
	Limit   int
	Page    int
	PerPage int
}

type UsersOptions struct {
	ListOptions
	Role string
}

type TicketCommentsOptions struct {
	TicketID string
	ListOptions
}

type OrganizationMembershipOptions struct {
	OrganizationID string
	UserID         string
	ListOptions
}

type SearchOptions struct {
	Query     string
	Include   string
	SortBy    string
	SortOrder string
	Page      int
	PerPage   int
}

type SearchExportOptions struct {
	Type  string
	Query string
	After string
	Limit int
}

type DownloadedAttachment struct {
	Metadata    map[string]any
	FileName    string
	ContentType string
	ContentURL  string
	Size        int64
	Body        []byte
}

type Client struct {
	httpClient *http.Client
	baseURL    string
	authHeader string
}

func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{httpClient: httpClient}
}

func NewAuthenticatedClient(instanceURL string, resolved config.ResolvedToken, httpClient *http.Client) (*Client, error) {
	client := NewClient(httpClient)

	baseURL := strings.TrimRight(strings.TrimSpace(instanceURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("instance URL is required")
	}
	if resolved.AuthType != config.AuthTypeAPIToken {
		return nil, fmt.Errorf("unsupported auth type %q", resolved.AuthType)
	}
	if strings.TrimSpace(resolved.Email) == "" {
		return nil, fmt.Errorf("email is required for api_token auth")
	}
	if strings.TrimSpace(resolved.Token) == "" {
		return nil, fmt.Errorf("api token is required")
	}

	client.baseURL = baseURL
	client.authHeader = basicAuthHeader(resolved.Email, resolved.Token)
	return client, nil
}

func (c *Client) GetTicket(ctx context.Context, id string) (map[string]any, error) {
	return c.getObject(ctx, "/api/v2/tickets/"+url.PathEscape(strings.TrimSpace(id)), nil, "ticket")
}

func (c *Client) ListTickets(ctx context.Context, opts ListOptions) (ListResponse, error) {
	values, mode, err := cursorPreferredQuery(opts, defaultCursorLimit, maxCursorPageSize)
	if err != nil {
		return ListResponse{}, err
	}
	return c.getList(ctx, "/api/v2/tickets", values, mode, "tickets")
}

func (c *Client) GetUser(ctx context.Context, id string) (map[string]any, error) {
	return c.getObject(ctx, "/api/v2/users/"+url.PathEscape(strings.TrimSpace(id)), nil, "user")
}

func (c *Client) ListUsers(ctx context.Context, opts UsersOptions) (ListResponse, error) {
	values, mode, err := cursorPreferredQuery(opts.ListOptions, defaultCursorLimit, maxCursorPageSize)
	if err != nil {
		return ListResponse{}, err
	}
	if role := strings.TrimSpace(opts.Role); role != "" {
		values.Set("role", role)
	}
	return c.getList(ctx, "/api/v2/users", values, mode, "users")
}

func (c *Client) GetOrganization(ctx context.Context, id string) (map[string]any, error) {
	return c.getObject(ctx, "/api/v2/organizations/"+url.PathEscape(strings.TrimSpace(id)), nil, "organization")
}

func (c *Client) GetAttachment(ctx context.Context, id string) (map[string]any, error) {
	return c.getObject(ctx, "/api/v2/attachments/"+url.PathEscape(strings.TrimSpace(id)), nil, "attachment")
}

func (c *Client) ListOrganizations(ctx context.Context, opts ListOptions) (ListResponse, error) {
	values, mode, err := cursorPreferredQuery(opts, defaultCursorLimit, maxCursorPageSize)
	if err != nil {
		return ListResponse{}, err
	}
	return c.getList(ctx, "/api/v2/organizations", values, mode, "organizations")
}

func (c *Client) ListOrganizationMemberships(ctx context.Context, opts OrganizationMembershipOptions) (ListResponse, error) {
	values, mode, err := cursorPreferredQuery(opts.ListOptions, defaultCursorLimit, maxCursorPageSize)
	if err != nil {
		return ListResponse{}, err
	}

	path := "/api/v2/organization_memberships"
	switch {
	case strings.TrimSpace(opts.OrganizationID) != "" && strings.TrimSpace(opts.UserID) != "":
		return ListResponse{}, fmt.Errorf("organization_id and user_id are mutually exclusive")
	case strings.TrimSpace(opts.OrganizationID) != "":
		path = "/api/v2/organizations/" + url.PathEscape(strings.TrimSpace(opts.OrganizationID)) + "/organization_memberships"
	case strings.TrimSpace(opts.UserID) != "":
		path = "/api/v2/users/" + url.PathEscape(strings.TrimSpace(opts.UserID)) + "/organization_memberships"
	}

	return c.getList(ctx, path, values, mode, "organization_memberships")
}

func (c *Client) ListTicketComments(ctx context.Context, opts TicketCommentsOptions) (ListResponse, error) {
	ticketID := strings.TrimSpace(opts.TicketID)
	if ticketID == "" {
		return ListResponse{}, fmt.Errorf("ticket_id is required")
	}

	values, mode, err := cursorPreferredQuery(opts.ListOptions, defaultCursorLimit, maxCursorPageSize)
	if err != nil {
		return ListResponse{}, err
	}

	path := "/api/v2/tickets/" + url.PathEscape(ticketID) + "/comments"
	return c.getList(ctx, path, values, mode, "comments")
}

func (c *Client) Search(ctx context.Context, opts SearchOptions) (ListResponse, error) {
	query := strings.TrimSpace(opts.Query)
	if query == "" {
		return ListResponse{}, fmt.Errorf("query is required")
	}

	values := url.Values{}
	values.Set("query", query)
	values.Set("page", strconv.Itoa(normalizePage(opts.Page)))
	values.Set("per_page", strconv.Itoa(normalizeBoundedSize(opts.PerPage, defaultOffsetPerPage, maxOffsetPerPage)))
	if include := strings.TrimSpace(opts.Include); include != "" {
		values.Set("include", include)
	}
	if sortBy := strings.TrimSpace(opts.SortBy); sortBy != "" {
		values.Set("sort_by", sortBy)
	}
	if sortOrder := strings.TrimSpace(opts.SortOrder); sortOrder != "" {
		values.Set("sort_order", sortOrder)
	}

	return c.getList(ctx, "/api/v2/search", values, "offset", "results")
}

func (c *Client) SearchCount(ctx context.Context, query string) (int, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return 0, fmt.Errorf("query is required")
	}

	values := url.Values{}
	values.Set("query", query)

	body, _, err := c.doRequest(ctx, "/api/v2/search/count", values)
	if err != nil {
		return 0, err
	}

	var payload struct {
		Count any `json:"count"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0, fmt.Errorf("decode search count response: %w", err)
	}

	switch count := payload.Count.(type) {
	case float64:
		return int(count), nil
	case map[string]any:
		if value, ok := count["value"].(float64); ok {
			return int(value), nil
		}
	}

	return 0, fmt.Errorf("search count response did not include a numeric count")
}

func (c *Client) SearchExport(ctx context.Context, opts SearchExportOptions) (ListResponse, error) {
	query := strings.TrimSpace(opts.Query)
	if query == "" {
		return ListResponse{}, fmt.Errorf("query is required")
	}
	filterType := strings.TrimSpace(opts.Type)
	if filterType == "" {
		return ListResponse{}, fmt.Errorf("type is required")
	}

	values := url.Values{}
	values.Set("query", query)
	values.Set("filter[type]", filterType)
	values.Set("page[size]", strconv.Itoa(normalizeBoundedSize(opts.Limit, defaultSearchExportSize, maxSearchExportPageSize)))
	if after := strings.TrimSpace(opts.After); after != "" {
		values.Set("page[after]", after)
	}

	return c.getList(ctx, "/api/v2/search/export", values, "cursor", "results")
}

func (c *Client) DownloadAttachment(ctx context.Context, id string) (DownloadedAttachment, error) {
	metadata, err := c.GetAttachment(ctx, id)
	if err != nil {
		return DownloadedAttachment{}, err
	}

	contentURL := stringValue(metadata["content_url"])
	if strings.TrimSpace(contentURL) == "" {
		return DownloadedAttachment{}, fmt.Errorf("attachment %q does not have a content_url", id)
	}
	if stringValue(metadata["malware_scan_result"]) == "malware_found" && !boolValue(metadata["malware_access_override"]) {
		return DownloadedAttachment{}, fmt.Errorf("attachment %q is flagged as malware and cannot be downloaded without override", id)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, contentURL, nil)
	if err != nil {
		return DownloadedAttachment{}, fmt.Errorf("build attachment download request: %w", err)
	}
	if sameHost(c.baseURL, contentURL) {
		req.Header.Set("Authorization", c.authHeader)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return DownloadedAttachment{}, fmt.Errorf("execute attachment download request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return DownloadedAttachment{}, fmt.Errorf("read attachment download response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return DownloadedAttachment{}, &HTTPError{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(body)),
			Headers:    cloneHeaders(resp.Header),
		}
	}

	fileName := stringValue(metadata["file_name"])
	if fileName == "" {
		fileName = strings.TrimSpace(id)
	}

	return DownloadedAttachment{
		Metadata:    metadata,
		FileName:    fileName,
		ContentType: stringValue(metadata["content_type"]),
		ContentURL:  contentURL,
		Size:        int64Value(metadata["size"]),
		Body:        body,
	}, nil
}

func (c *Client) doRequest(ctx context.Context, path string, query url.Values) ([]byte, http.Header, error) {
	if c == nil || c.httpClient == nil {
		return nil, nil, fmt.Errorf("http client is not configured")
	}
	if strings.TrimSpace(c.baseURL) == "" {
		return nil, nil, fmt.Errorf("base URL is not configured")
	}

	target := c.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if c.authHeader != "" {
		req.Header.Set("Authorization", c.authHeader)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.Header.Clone(), &HTTPError{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(body)),
			Headers:    cloneHeaders(resp.Header),
		}
	}

	return body, resp.Header.Clone(), nil
}

func (c *Client) getObject(ctx context.Context, path string, query url.Values, rootKey string) (map[string]any, error) {
	body, _, err := c.doRequest(ctx, path, query)
	if err != nil {
		return nil, err
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode %s response: %w", rootKey, err)
	}

	object, ok := payload[rootKey].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("response did not include %q object", rootKey)
	}
	return object, nil
}

func (c *Client) getList(ctx context.Context, path string, query url.Values, mode string, rootKey string) (ListResponse, error) {
	body, _, err := c.doRequest(ctx, path, query)
	if err != nil {
		return ListResponse{}, err
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return ListResponse{}, fmt.Errorf("decode %s response: %w", rootKey, err)
	}

	items, err := toMapSlice(payload[rootKey])
	if err != nil {
		return ListResponse{}, fmt.Errorf("decode %s items: %w", rootKey, err)
	}

	page := extractPageInfo(payload, mode)
	return ListResponse{
		Items: items,
		Page:  page,
	}, nil
}

func cursorPreferredQuery(opts ListOptions, defaultSize, maxSize int) (url.Values, string, error) {
	values := url.Values{}
	if opts.Page > 0 || opts.PerPage > 0 {
		values.Set("page", strconv.Itoa(normalizePage(opts.Page)))
		values.Set("per_page", strconv.Itoa(normalizeBoundedSize(opts.PerPage, defaultOffsetPerPage, maxOffsetPerPage)))
		return values, "offset", nil
	}

	values.Set("page[size]", strconv.Itoa(normalizeBoundedSize(opts.Limit, defaultSize, maxSize)))
	if after := strings.TrimSpace(opts.After); after != "" {
		values.Set("page[after]", after)
	}
	return values, "cursor", nil
}

func normalizePage(page int) int {
	if page <= 0 {
		return 1
	}
	return page
}

func normalizeBoundedSize(value, defaultSize, maxSize int) int {
	if value <= 0 {
		return defaultSize
	}
	if value > maxSize {
		return maxSize
	}
	return value
}

func toMapSlice(value any) ([]map[string]any, error) {
	if value == nil {
		return []map[string]any{}, nil
	}

	rawItems, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("value is %T, want array", value)
	}

	items := make([]map[string]any, 0, len(rawItems))
	for _, rawItem := range rawItems {
		item, ok := rawItem.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("array item is %T, want object", rawItem)
		}
		items = append(items, item)
	}
	return items, nil
}

func extractPageInfo(payload map[string]any, mode string) PageInfo {
	page := PageInfo{Mode: mode}

	if nextPage, ok := payload["next_page"].(string); ok {
		page.NextPage = nextPage
	}
	if previousPage, ok := payload["previous_page"].(string); ok {
		page.PreviousPage = previousPage
	}
	if count, ok := payload["count"].(float64); ok {
		value := int(count)
		page.Count = &value
	}

	if links, ok := payload["links"].(map[string]any); ok {
		if nextValue, ok := links["next"].(string); ok {
			page.NextPage = nextValue
		}
		if previousValue, ok := links["prev"].(string); ok {
			page.PreviousPage = previousValue
		}
	}

	if meta, ok := payload["meta"].(map[string]any); ok {
		if hasMore, ok := meta["has_more"].(bool); ok {
			page.HasMore = hasMore
		}
		if afterCursor, ok := meta["after_cursor"].(string); ok {
			page.AfterCursor = afterCursor
		}
	}

	return page
}

func cloneHeaders(headers http.Header) map[string]string {
	if len(headers) == 0 {
		return nil
	}

	out := make(map[string]string, len(headers))
	for key, values := range headers {
		if len(values) == 0 {
			continue
		}
		out[key] = strings.Join(values, ", ")
	}
	return out
}

func basicAuthHeader(email, token string) string {
	value := strings.TrimSpace(email) + "/token:" + strings.TrimSpace(token)
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(value))
}

func sameHost(leftURL, rightURL string) bool {
	left, err := url.Parse(leftURL)
	if err != nil {
		return false
	}
	right, err := url.Parse(rightURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(left.Host, right.Host)
}

package zendesk

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/relux-works/skill-zendesk-management/internal/config"
)

func TestQueryEngineSchema(t *testing.T) {
	engine := NewQueryEngine(&Client{httpClient: http.DefaultClient, baseURL: "https://example.zendesk.com", authHeader: "Basic test"})

	results, err := engine.Execute(context.Background(), "schema()")
	if err != nil {
		t.Fatalf("Execute(schema) error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}

	value := results[0].jsonValue()
	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("schema result type = %T, want object", value)
	}

	operations, ok := object["operations"].([]string)
	if !ok {
		t.Fatalf("operations type = %T, want []string", object["operations"])
	}
	if len(operations) == 0 {
		t.Fatal("operations should not be empty")
	}
}

func TestQueryEngineTicketAndSearchUseZendeskTransport(t *testing.T) {
	var ticketAuthHeader string
	var searchQuery string
	var searchPage string
	var searchPerPage string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/tickets/123":
			ticketAuthHeader = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ticket":{"id":123,"subject":"Broken sync","status":"open","priority":"high","type":"incident","requester_id":7,"assignee_id":9,"organization_id":3,"updated_at":"2026-04-10T12:00:00Z","tags":["sync"]}}`))
		case "/api/v2/tickets/123/comments":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"comments":[{"id":10,"author_id":7,"public":true,"created_at":"2026-04-10T12:00:00Z","attachments":[{"id":498483,"file_name":"trace.log","size":2532,"content_type":"text/plain"},{"id":498484,"file_name":"sync.png","size":1200,"content_type":"image/png"}]}],"meta":{"has_more":false,"after_cursor":""},"links":{"next":"","prev":""}}`))
		case "/api/v2/search":
			searchQuery = r.URL.Query().Get("query")
			searchPage = r.URL.Query().Get("page")
			searchPerPage = r.URL.Query().Get("per_page")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"results":[{"result_type":"ticket","id":123,"subject":"Broken sync","status":"open","priority":"high","requester_id":7,"updated_at":"2026-04-10T12:00:00Z","url":"https://example.zendesk.com/api/v2/tickets/123"}],"count":1,"next_page":"https://example.zendesk.com/api/v2/search?page=3","previous_page":"https://example.zendesk.com/api/v2/search?page=1"}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewAuthenticatedClient(server.URL, config.ResolvedToken{
		Email:    "agent@example.com",
		Token:    "test-token",
		AuthType: config.AuthTypeAPIToken,
	}, server.Client())
	if err != nil {
		t.Fatalf("NewAuthenticatedClient() error = %v", err)
	}

	engine := NewQueryEngine(client)

	results, err := engine.Execute(context.Background(), `ticket(123) { overview }; search(query="type:ticket broken", page=2, per_page=5) { minimal }`)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}

	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("agent@example.com/token:test-token"))
	if ticketAuthHeader != wantAuth {
		t.Fatalf("Authorization = %q, want %q", ticketAuthHeader, wantAuth)
	}

	ticketObject := results[0].Object
	if ticketObject["id"] != float64(123) {
		t.Fatalf("ticket id = %#v, want 123", ticketObject["id"])
	}
	attachments, ok := ticketObject["attachments"].([]map[string]any)
	if !ok {
		t.Fatalf("ticket attachments type = %T, want []map[string]any", ticketObject["attachments"])
	}
	if len(attachments) != 2 {
		t.Fatalf("ticket attachments len = %d, want 2", len(attachments))
	}
	if got := attachments[0]["file_name"]; got != "trace.log" {
		t.Fatalf("ticket attachment file_name = %#v, want trace.log", got)
	}
	if _, ok := ticketObject["tags"]; !ok {
		t.Fatalf("ticket overview should include tags: %#v", ticketObject)
	}
	if _, ok := ticketObject["url"]; ok {
		t.Fatalf("ticket overview should not include url: %#v", ticketObject)
	}

	if searchQuery != "type:ticket broken" || searchPage != "2" || searchPerPage != "5" {
		t.Fatalf("search transport = query=%q page=%q per_page=%q", searchQuery, searchPage, searchPerPage)
	}

	if results[1].Kind != ResultKindList || len(results[1].Items) != 1 {
		t.Fatalf("search result = %#v", results[1])
	}
	if got := results[1].Items[0]["result_type"]; got != "ticket" {
		t.Fatalf("search result_type = %#v, want ticket", got)
	}

	compact, err := RenderCompact([]Result{results[1]})
	if err != nil {
		t.Fatalf("RenderCompact() error = %v", err)
	}
	if compact == "" {
		t.Fatal("compact output should not be empty")
	}

	ticketCompact, err := RenderCompact([]Result{results[0]})
	if err != nil {
		t.Fatalf("RenderCompact(ticket) error = %v", err)
	}
	if want := "attachments:498483 trace.log | 498484 sync.png"; !containsLine(ticketCompact, want) {
		t.Fatalf("ticket compact output missing %q:\n%s", want, ticketCompact)
	}
}

func TestQueryEngineAttachmentAndTicketAttachments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/attachments/498483":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"attachment":{"id":498483,"file_name":"trace.log","size":2532,"content_type":"text/plain","content_url":"https://example.zendesk.com/files/trace.log","malware_scan_result":"malware_not_found"}}`))
		case "/api/v2/tickets/123/comments":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"comments":[{"id":10,"author_id":7,"public":true,"created_at":"2026-04-10T12:00:00Z","attachments":[{"id":498483,"file_name":"trace.log","size":2532,"content_type":"text/plain","content_url":"https://example.zendesk.com/files/trace.log","malware_scan_result":"malware_not_found"}]}]}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewAuthenticatedClient(server.URL, config.ResolvedToken{
		Email:    "agent@example.com",
		Token:    "test-token",
		AuthType: config.AuthTypeAPIToken,
	}, server.Client())
	if err != nil {
		t.Fatalf("NewAuthenticatedClient() error = %v", err)
	}

	engine := NewQueryEngine(client)
	results, err := engine.Execute(context.Background(), `attachment(498483) { minimal }; ticket_attachments(ticket_id=123) { overview }`)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}

	if got := results[0].Object["file_name"]; got != "trace.log" {
		t.Fatalf("attachment file_name = %#v, want trace.log", got)
	}
	if len(results[1].Items) != 1 {
		t.Fatalf("ticket_attachments len = %d, want 1", len(results[1].Items))
	}
	if got := results[1].Items[0]["comment_id"]; got != float64(10) {
		t.Fatalf("ticket attachment comment_id = %#v, want 10", got)
	}
	if got := results[1].Items[0]["ticket_id"]; got != "123" {
		t.Fatalf("ticket attachment ticket_id = %#v, want 123", got)
	}
}

func TestCompactAttachmentRefs(t *testing.T) {
	value := []map[string]any{
		{"id": float64(498483), "file_name": "trace.log"},
		{"id": float64(498484), "file_name": "sync.png"},
	}

	got := compactValue(value)
	want := "498483 trace.log | 498484 sync.png"
	if got != want {
		t.Fatalf("compactValue(attachmentRefs) = %q, want %q", got, want)
	}
}

func TestQueryEngineUnsupportedOperationIncludesGuidance(t *testing.T) {
	engine := NewQueryEngine(&Client{httpClient: http.DefaultClient, baseURL: "https://example.zendesk.com", authHeader: "Basic test"})

	_, err := engine.Execute(context.Background(), `get(68218) { id }`)
	if err == nil {
		t.Fatal("Execute(get) error = nil, want guidance error")
	}
	if !strings.Contains(err.Error(), `unsupported operation "get"`) {
		t.Fatalf("error = %v, want unsupported operation guidance", err)
	}
	if !strings.Contains(err.Error(), "ticket(ID)") || !strings.Contains(err.Error(), "ticket_comments(ticket_id=ID)") {
		t.Fatalf("error = %v, want ticket/ticket_comments guidance", err)
	}
}

func containsLine(text, line string) bool {
	for _, candidate := range strings.Split(text, "\n") {
		if candidate == line {
			return true
		}
	}
	return false
}

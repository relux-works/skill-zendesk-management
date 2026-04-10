package zendesk

import "testing"

func TestParseQueryBatchSupportsBatchingAndQuotedParams(t *testing.T) {
	requests, err := ParseQueryBatch(`ticket(12345) { overview }; search(query="type:ticket status:open", sort_by=updated_at, sort_order=desc) { minimal }`)
	if err != nil {
		t.Fatalf("ParseQueryBatch() error = %v", err)
	}

	if len(requests) != 2 {
		t.Fatalf("len(requests) = %d, want 2", len(requests))
	}

	if requests[0].Operation != "ticket" || requests[0].Positional != "12345" {
		t.Fatalf("first request = %#v", requests[0])
	}
	if len(requests[0].Fields) != 1 || requests[0].Fields[0] != "overview" {
		t.Fatalf("first request fields = %#v, want overview", requests[0].Fields)
	}

	if requests[1].Operation != "search" {
		t.Fatalf("second request operation = %q, want search", requests[1].Operation)
	}
	if got := requests[1].Params["query"]; got != "type:ticket status:open" {
		t.Fatalf("search query param = %q, want %q", got, "type:ticket status:open")
	}
	if got := requests[1].Params["sort_by"]; got != "updated_at" {
		t.Fatalf("search sort_by = %q, want updated_at", got)
	}
	if got := requests[1].Params["sort_order"]; got != "desc" {
		t.Fatalf("search sort_order = %q, want desc", got)
	}
	if len(requests[1].Fields) != 1 || requests[1].Fields[0] != "minimal" {
		t.Fatalf("second request fields = %#v, want minimal", requests[1].Fields)
	}
}

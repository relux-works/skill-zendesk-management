package zendesk

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/relux-works/skill-zendesk-management/internal/config"
)

func TestDownloadAttachmentUsesAuthOnlyForSameHost(t *testing.T) {
	var sameHostAuthHeader string
	var externalAuthHeader string
	var sameHostFileURL string

	externalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		externalAuthHeader = r.Header.Get("Authorization")
		_, _ = w.Write([]byte("external-body"))
	}))
	defer externalServer.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/attachments/1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"attachment":{"id":1,"file_name":"same.txt","size":9,"content_type":"text/plain","content_url":"` + sameHostFileURL + `","malware_scan_result":"malware_not_found"}}`))
		case "/api/v2/attachments/2":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"attachment":{"id":2,"file_name":"external.txt","size":13,"content_type":"text/plain","content_url":"` + externalServer.URL + `/download.txt","malware_scan_result":"malware_not_found"}}`))
		case "/files/same.txt":
			sameHostAuthHeader = r.Header.Get("Authorization")
			_, _ = w.Write([]byte("same-body"))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()
	sameHostFileURL = server.URL + "/files/same.txt"

	client, err := NewAuthenticatedClient(server.URL, config.ResolvedToken{
		Email:    "agent@example.com",
		Token:    "test-token",
		AuthType: config.AuthTypeAPIToken,
	}, server.Client())
	if err != nil {
		t.Fatalf("NewAuthenticatedClient() error = %v", err)
	}

	gotSameHost, err := client.DownloadAttachment(context.Background(), "1")
	if err != nil {
		t.Fatalf("DownloadAttachment(same host) error = %v", err)
	}
	if string(gotSameHost.Body) != "same-body" {
		t.Fatalf("same host body = %q, want %q", string(gotSameHost.Body), "same-body")
	}

	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("agent@example.com/token:test-token"))
	if sameHostAuthHeader != wantAuth {
		t.Fatalf("same host Authorization = %q, want %q", sameHostAuthHeader, wantAuth)
	}

	gotExternal, err := client.DownloadAttachment(context.Background(), "2")
	if err != nil {
		t.Fatalf("DownloadAttachment(external) error = %v", err)
	}
	if string(gotExternal.Body) != "external-body" {
		t.Fatalf("external body = %q, want %q", string(gotExternal.Body), "external-body")
	}
	if externalAuthHeader != "" {
		t.Fatalf("external Authorization = %q, want empty", externalAuthHeader)
	}
}

package zendesk

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/relux-works/skill-zendesk-management/internal/config"
)

func TestCheckAuthUsesBasicAuthAndParsesUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/users/me.json" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/api/v2/users/me.json")
		}

		want := "Basic " + base64.StdEncoding.EncodeToString([]byte("agent@example.com/token:test-token"))
		if got := r.Header.Get("Authorization"); got != want {
			t.Fatalf("Authorization = %q, want %q", got, want)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"user":{"id":42,"name":"Agent","email":"agent@example.com","role":"admin","active":true,"suspended":false}}`))
	}))
	defer server.Close()

	client := NewClient(server.Client())
	got, err := client.CheckAuth(context.Background(), server.URL, config.ResolvedToken{
		Email:    "agent@example.com",
		Token:    "test-token",
		AuthType: config.AuthTypeAPIToken,
	})
	if err != nil {
		t.Fatalf("CheckAuth() error = %v", err)
	}

	if got.HTTPStatus != http.StatusOK {
		t.Fatalf("HTTPStatus = %d, want %d", got.HTTPStatus, http.StatusOK)
	}
	if got.UserID != 42 || got.Email != "agent@example.com" || got.Role != "admin" || !got.Active || got.Suspended {
		t.Fatalf("unexpected result: %#v", got)
	}
}

func TestCheckAuthReturnsHTTPErrorOnUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"title":"Unauthorized","message":"Couldn't authenticate you"}}`))
	}))
	defer server.Close()

	client := NewClient(server.Client())
	_, err := client.CheckAuth(context.Background(), server.URL, config.ResolvedToken{
		Email:    "agent@example.com",
		Token:    "bad-token",
		AuthType: config.AuthTypeAPIToken,
	})
	if err == nil {
		t.Fatal("CheckAuth() error = nil, want *HTTPError")
	}

	httpErr, ok := err.(*HTTPError)
	if !ok {
		t.Fatalf("error type = %T, want *HTTPError", err)
	}
	if httpErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("StatusCode = %d, want %d", httpErr.StatusCode, http.StatusUnauthorized)
	}
}

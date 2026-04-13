package artifacts

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/relux-works/skill-zendesk-management/internal/zendesk"
)

type fakeClient struct {
	comments   []map[string]any
	downloads  map[string]zendesk.DownloadedAttachment
	afterCalls []string
}

func (f *fakeClient) ListTicketComments(_ context.Context, opts zendesk.TicketCommentsOptions) (zendesk.ListResponse, error) {
	f.afterCalls = append(f.afterCalls, opts.After)
	return zendesk.ListResponse{
		Items: f.comments,
		Page:  zendesk.PageInfo{HasMore: false},
	}, nil
}

func (f *fakeClient) DownloadAttachment(_ context.Context, id string) (zendesk.DownloadedAttachment, error) {
	return f.downloads[id], nil
}

func TestMaterializeTicketExpandsArchivesAndSanitizesText(t *testing.T) {
	zipBody := buildZip(t, map[string]string{
		"logs/app.log":            "password=supersecret\nhttps://server01.internal.example/share/folder1/file.txt\n",
		"nested/config.json":      "{\"token\":\"abc123xyz\"}\n",
		"screenshots/view.png":    string([]byte{0x89, 'P', 'N', 'G'}),
		"nested/windows.log":      "Path=C:\\Users\\alexis\\Desktop\\secret\\app.log\n",
		"nested/contacts.txt":     "owner=agent@example.com\nhost=server01.internal.example\n",
		"nested/metrics/data.tsv": "service\t10.0.0.42\n",
	})

	client := &fakeClient{
		comments: []map[string]any{
			{
				"id":         float64(10),
				"created_at": "2026-04-14T12:00:00Z",
				"public":     true,
				"attachments": []any{
					map[string]any{
						"id":           float64(1),
						"file_name":    "support-bundle.zip",
						"content_type": "application/zip",
					},
					map[string]any{
						"id":           float64(2),
						"file_name":    "trace.log",
						"content_type": "text/plain",
					},
				},
			},
		},
		downloads: map[string]zendesk.DownloadedAttachment{
			"1": {
				FileName:    "support-bundle.zip",
				ContentType: "application/zip",
				Body:        zipBody,
			},
			"2": {
				FileName:    "trace.log",
				ContentType: "text/plain",
				Body:        []byte("Authorization: Bearer very-secret-token\nlogin=alexis\n"),
			},
		},
	}

	rootDir := filepath.Join(t.TempDir(), ".attachments")
	result, err := NewMaterializer(client).MaterializeTicket(context.Background(), MaterializeOptions{
		TicketID: "12345",
		RootDir:  rootDir,
	})
	if err != nil {
		t.Fatalf("MaterializeTicket() error = %v", err)
	}

	if result.AttachmentCount != 2 {
		t.Fatalf("AttachmentCount = %d, want 2", result.AttachmentCount)
	}

	topLevelLog := filepath.Join(rootDir, "files", "002-trace.log")
	gotTopLevel, err := os.ReadFile(topLevelLog)
	if err != nil {
		t.Fatalf("read top-level log: %v", err)
	}
	if strings.Contains(string(gotTopLevel), "very-secret-token") {
		t.Fatalf("top-level log leaked raw token: %s", string(gotTopLevel))
	}
	if !strings.Contains(string(gotTopLevel), "<token:") {
		t.Fatalf("top-level log missing token redaction: %s", string(gotTopLevel))
	}

	extractedLog := filepath.Join(rootDir, "expanded", "001-support-bundle", "logs", "app.log")
	gotExtracted, err := os.ReadFile(extractedLog)
	if err != nil {
		t.Fatalf("read extracted log: %v", err)
	}
	text := string(gotExtracted)
	if strings.Contains(text, "supersecret") || strings.Contains(text, "server01.internal.example") {
		t.Fatalf("extracted log leaked raw values: %s", text)
	}
	if !strings.Contains(text, "<secret:") || !strings.Contains(text, "<host:") {
		t.Fatalf("extracted log missing redaction markers: %s", text)
	}

	binaryPath := filepath.Join(rootDir, "expanded", "001-support-bundle", "screenshots", "view.png")
	if _, err := os.Stat(binaryPath); err != nil {
		t.Fatalf("binary archive entry missing: %v", err)
	}

	manifest, err := os.ReadFile(filepath.Join(rootDir, ManifestFileName))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	manifestText := string(manifest)
	if !strings.Contains(manifestText, `"ticket_id": "12345"`) {
		t.Fatalf("manifest missing ticket id: %s", manifestText)
	}
	if !strings.Contains(manifestText, `"path": "expanded/001-support-bundle/logs/app.log"`) {
		t.Fatalf("manifest missing extracted path: %s", manifestText)
	}
}

func TestRedactorSanitizeTextRedactsStructuredValues(t *testing.T) {
	redactor := NewRedactor("test-salt")
	input := strings.Join([]string{
		"Authorization: Bearer very-secret-token",
		"password=supersecret",
		"user=alexis",
		"owner=agent@example.com",
		"host=server01.internal.example",
		"https://server01.internal.example/share/folder1/file.txt",
		`C:\Users\alexis\Desktop\secret\app.log`,
		"/Users/alexis/Library/Application Support/app/config.yaml",
		"10.0.0.42",
	}, "\n")

	output := redactor.SanitizeText(input)
	for _, raw := range []string{
		"very-secret-token",
		"supersecret",
		"agent@example.com",
		"server01.internal.example",
		"10.0.0.42",
		`C:\Users\alexis\Desktop\secret\app.log`,
		"/Users/alexis/Library/Application Support/app/config.yaml",
	} {
		if strings.Contains(output, raw) {
			t.Fatalf("output leaked %q:\n%s", raw, output)
		}
	}
	for _, marker := range []string{"<token:", "<secret:", "<login:", "<email:", "<host:", "<ip:", "<seg:"} {
		if !strings.Contains(output, marker) {
			t.Fatalf("output missing marker %q:\n%s", marker, output)
		}
	}
}

func buildZip(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("Create(%q) error = %v", name, err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatalf("Write(%q) error = %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return buf.Bytes()
}

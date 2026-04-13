package artifacts

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/relux-works/skill-zendesk-management/internal/zendesk"
)

const (
	DefaultRootDir     = ".attachments"
	ManifestFileName   = "manifest.json"
	redactionSaltFile  = ".redaction-salt"
	defaultCommentPage = 100
	maxArchiveDepth    = 4
)

type Client interface {
	ListTicketComments(ctx context.Context, opts zendesk.TicketCommentsOptions) (zendesk.ListResponse, error)
	DownloadAttachment(ctx context.Context, id string) (zendesk.DownloadedAttachment, error)
}

type Materializer struct {
	client Client
}

type MaterializeOptions struct {
	TicketID string
	RootDir  string
	Force    bool
}

type Result struct {
	TicketID        string   `json:"ticket_id"`
	RootDir         string   `json:"root_dir"`
	ManifestPath    string   `json:"manifest_path"`
	SearchRoots     []string `json:"search_roots"`
	AttachmentCount int      `json:"attachment_count"`
	ArtifactCount   int      `json:"artifact_count"`
}

type Manifest struct {
	TicketID        string            `json:"ticket_id"`
	GeneratedAt     string            `json:"generated_at"`
	RootDir         string            `json:"root_dir"`
	SearchRoots     []string          `json:"search_roots"`
	AttachmentCount int               `json:"attachment_count"`
	ArtifactCount   int               `json:"artifact_count"`
	Attachments     []AttachmentEntry `json:"attachments"`
}

type AttachmentEntry struct {
	AttachmentID     string           `json:"attachment_id"`
	FileName         string           `json:"file_name"`
	ContentType      string           `json:"content_type,omitempty"`
	CommentID        string           `json:"comment_id,omitempty"`
	CommentCreatedAt string           `json:"comment_created_at,omitempty"`
	CommentPublic    bool             `json:"comment_public"`
	Kind             string           `json:"kind"`
	Outputs          []ArtifactOutput `json:"outputs"`
	Warning          string           `json:"warning,omitempty"`
}

type ArtifactOutput struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Sanitized  bool   `json:"sanitized"`
	Searchable bool   `json:"searchable"`
}

type attachmentRef struct {
	ID               string
	FileName         string
	ContentType      string
	CommentID        string
	CommentCreatedAt string
	CommentPublic    bool
}

type archiveEntry struct {
	Name        string
	ContentType string
	Body        []byte
}

func NewMaterializer(client Client) *Materializer {
	return &Materializer{client: client}
}

func (m *Materializer) MaterializeTicket(ctx context.Context, opts MaterializeOptions) (Result, error) {
	if m == nil || m.client == nil {
		return Result{}, fmt.Errorf("materializer client is not configured")
	}

	ticketID := strings.TrimSpace(opts.TicketID)
	if ticketID == "" {
		return Result{}, fmt.Errorf("ticket id is required")
	}

	rootDir := strings.TrimSpace(opts.RootDir)
	if rootDir == "" {
		rootDir = DefaultRootDir
	}
	rootDir = filepath.Clean(rootDir)

	salt, err := prepareRootDir(rootDir, opts.Force)
	if err != nil {
		return Result{}, err
	}

	refs, err := listAllTicketAttachments(ctx, m.client, ticketID)
	if err != nil {
		return Result{}, err
	}

	redactor := NewRedactor(salt)
	manifest := Manifest{
		TicketID:    ticketID,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		RootDir:     filepath.ToSlash(rootDir),
		SearchRoots: []string{"files", "expanded"},
	}

	for idx, ref := range refs {
		downloaded, err := m.client.DownloadAttachment(ctx, ref.ID)
		if err != nil {
			return Result{}, fmt.Errorf("download attachment %s: %w", ref.ID, err)
		}

		fileName := strings.TrimSpace(downloaded.FileName)
		if fileName == "" {
			fileName = ref.FileName
		}
		contentType := strings.TrimSpace(downloaded.ContentType)
		if contentType == "" {
			contentType = ref.ContentType
		}

		prefixedName := prefixedSafeName(idx+1, fileName)
		entry := AttachmentEntry{
			AttachmentID:     ref.ID,
			FileName:         fileName,
			ContentType:      contentType,
			CommentID:        ref.CommentID,
			CommentCreatedAt: ref.CommentCreatedAt,
			CommentPublic:    ref.CommentPublic,
		}

		if isArchiveFile(fileName, contentType, downloaded.Body) {
			entry.Kind = "archive"
			outputs, extractErr := materializeArchive(rootDir, filepath.ToSlash(filepath.Join("expanded", trimArchiveExtension(prefixedName))), fileName, contentType, downloaded.Body, redactor, 0)
			if extractErr == nil && len(outputs) > 0 {
				entry.Outputs = outputs
			} else {
				output, writeErr := materializeFile(rootDir, filepath.ToSlash(filepath.Join("files", prefixedName)), fileName, contentType, downloaded.Body, redactor)
				if writeErr != nil {
					return Result{}, fmt.Errorf("write attachment %s: %w", ref.ID, writeErr)
				}
				entry.Kind = output.Kind
				entry.Outputs = []ArtifactOutput{output}
				if extractErr != nil {
					entry.Warning = "archive extraction failed; kept original file"
				}
			}
		} else {
			output, err := materializeFile(rootDir, filepath.ToSlash(filepath.Join("files", prefixedName)), fileName, contentType, downloaded.Body, redactor)
			if err != nil {
				return Result{}, fmt.Errorf("write attachment %s: %w", ref.ID, err)
			}
			entry.Kind = output.Kind
			entry.Outputs = []ArtifactOutput{output}
		}

		manifest.Attachments = append(manifest.Attachments, entry)
		manifest.ArtifactCount += len(entry.Outputs)
	}
	manifest.AttachmentCount = len(manifest.Attachments)

	manifestPath := filepath.Join(rootDir, ManifestFileName)
	if err := writeManifest(manifestPath, manifest); err != nil {
		return Result{}, err
	}

	return Result{
		TicketID:        ticketID,
		RootDir:         rootDir,
		ManifestPath:    manifestPath,
		SearchRoots:     manifest.SearchRoots,
		AttachmentCount: manifest.AttachmentCount,
		ArtifactCount:   manifest.ArtifactCount,
	}, nil
}

func prepareRootDir(rootDir string, force bool) (string, error) {
	existingSalt, _ := readSalt(filepath.Join(rootDir, redactionSaltFile))

	info, err := os.Stat(rootDir)
	switch {
	case err == nil && !info.IsDir():
		return "", fmt.Errorf("attachments destination is not a directory: %s", rootDir)
	case err == nil && force:
		if err := os.RemoveAll(rootDir); err != nil {
			return "", fmt.Errorf("remove existing attachments dir: %w", err)
		}
	case err == nil && !force:
		entries, readErr := os.ReadDir(rootDir)
		if readErr != nil {
			return "", fmt.Errorf("read attachments dir: %w", readErr)
		}
		if len(entries) > 0 {
			return "", fmt.Errorf("attachments destination already exists and is not empty: %s (use --force)", rootDir)
		}
	case err != nil && !os.IsNotExist(err):
		return "", fmt.Errorf("stat attachments dir: %w", err)
	}

	for _, rel := range []string{"files", "expanded"} {
		if err := os.MkdirAll(filepath.Join(rootDir, rel), 0o755); err != nil {
			return "", fmt.Errorf("create attachments subdir %s: %w", rel, err)
		}
	}

	saltPath := filepath.Join(rootDir, redactionSaltFile)
	if existingSalt != "" {
		if err := os.WriteFile(saltPath, []byte(existingSalt), 0o600); err != nil {
			return "", fmt.Errorf("write redaction salt: %w", err)
		}
		return existingSalt, nil
	}

	saltBytes := make([]byte, 16)
	if _, err := rand.Read(saltBytes); err != nil {
		return "", fmt.Errorf("generate redaction salt: %w", err)
	}
	salt := hex.EncodeToString(saltBytes)
	if err := os.WriteFile(saltPath, []byte(salt), 0o600); err != nil {
		return "", fmt.Errorf("write redaction salt: %w", err)
	}
	return salt, nil
}

func readSalt(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func writeManifest(path string, manifest Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

func listAllTicketAttachments(ctx context.Context, client Client, ticketID string) ([]attachmentRef, error) {
	var (
		after string
		refs  []attachmentRef
		seen  = map[string]struct{}{}
	)

	for {
		response, err := client.ListTicketComments(ctx, zendesk.TicketCommentsOptions{
			TicketID: ticketID,
			ListOptions: zendesk.ListOptions{
				After: after,
				Limit: defaultCommentPage,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("list ticket comments: %w", err)
		}

		for _, comment := range response.Items {
			commentID := toString(comment["id"])
			commentCreatedAt := toString(comment["created_at"])
			commentPublic := toBool(comment["public"])

			rawAttachments, ok := comment["attachments"].([]any)
			if !ok {
				continue
			}

			for _, rawAttachment := range rawAttachments {
				attachment, ok := rawAttachment.(map[string]any)
				if !ok {
					continue
				}
				id := toString(attachment["id"])
				if id == "" {
					continue
				}
				if _, exists := seen[id]; exists {
					continue
				}
				seen[id] = struct{}{}
				refs = append(refs, attachmentRef{
					ID:               id,
					FileName:         firstNonEmpty(toString(attachment["file_name"]), toString(attachment["name"])),
					ContentType:      toString(attachment["content_type"]),
					CommentID:        commentID,
					CommentCreatedAt: commentCreatedAt,
					CommentPublic:    commentPublic,
				})
			}
		}

		if !response.Page.HasMore || strings.TrimSpace(response.Page.AfterCursor) == "" {
			break
		}
		after = response.Page.AfterCursor
	}

	return refs, nil
}

func materializeArchive(rootDir, dirRel, logicalName, contentType string, body []byte, redactor *Redactor, depth int) ([]ArtifactOutput, error) {
	if depth >= maxArchiveDepth {
		return nil, fmt.Errorf("archive nesting exceeds max depth %d", maxArchiveDepth)
	}

	entries, err := extractArchiveEntries(logicalName, contentType, body)
	if err != nil {
		return nil, err
	}

	outputs := make([]ArtifactOutput, 0, len(entries))
	for _, entry := range entries {
		entryRel := sanitizeArchiveEntryPath(entry.Name)
		if entryRel == "" {
			continue
		}

		targetRel := filepath.ToSlash(filepath.Join(dirRel, entryRel))
		if isArchiveFile(entry.Name, entry.ContentType, entry.Body) {
			childOutputs, err := materializeArchive(rootDir, trimArchiveExtension(targetRel), entry.Name, entry.ContentType, entry.Body, redactor, depth+1)
			if err != nil {
				output, writeErr := materializeFile(rootDir, targetRel, entry.Name, entry.ContentType, entry.Body, redactor)
				if writeErr != nil {
					return nil, writeErr
				}
				outputs = append(outputs, output)
				continue
			}
			outputs = append(outputs, childOutputs...)
			continue
		}

		output, err := materializeFile(rootDir, targetRel, entry.Name, entry.ContentType, entry.Body, redactor)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, output)
	}

	return outputs, nil
}

func materializeFile(rootDir, relPath, logicalName, contentType string, body []byte, redactor *Redactor) (ArtifactOutput, error) {
	relPath = filepath.ToSlash(filepath.Clean(filepath.FromSlash(relPath)))
	fullPath := filepath.Join(rootDir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return ArtifactOutput{}, fmt.Errorf("create parent dir: %w", err)
	}

	if isTextLike(logicalName, contentType, body) {
		text := string(body)
		if !utf8.Valid(body) {
			text = strings.ToValidUTF8(string(body), "")
		}
		sanitized := redactor.SanitizeText(text)
		if err := os.WriteFile(fullPath, []byte(sanitized), 0o600); err != nil {
			return ArtifactOutput{}, fmt.Errorf("write sanitized file: %w", err)
		}
		return ArtifactOutput{
			Path:       relPath,
			Kind:       "text",
			Sanitized:  true,
			Searchable: true,
		}, nil
	}

	if err := os.WriteFile(fullPath, body, 0o600); err != nil {
		return ArtifactOutput{}, fmt.Errorf("write binary file: %w", err)
	}
	return ArtifactOutput{
		Path:       relPath,
		Kind:       "binary",
		Sanitized:  false,
		Searchable: false,
	}, nil
}

func extractArchiveEntries(name, contentType string, body []byte) ([]archiveEntry, error) {
	lowerName := strings.ToLower(strings.TrimSpace(name))
	lowerType := strings.ToLower(strings.TrimSpace(contentType))

	switch {
	case strings.HasSuffix(lowerName, ".zip") || strings.Contains(lowerType, "zip"):
		return readZipEntries(body)
	case strings.HasSuffix(lowerName, ".tar.gz"), strings.HasSuffix(lowerName, ".tgz"):
		return readTarGzEntries(body)
	case strings.HasSuffix(lowerName, ".tar") || strings.Contains(lowerType, "x-tar"):
		return readTarEntries(bytes.NewReader(body))
	case strings.HasSuffix(lowerName, ".gz") || strings.Contains(lowerType, "gzip"):
		return readGzipEntry(name, body)
	default:
		return nil, fmt.Errorf("unsupported archive type: %s", name)
	}
}

func readZipEntries(body []byte) ([]archiveEntry, error) {
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, fmt.Errorf("open zip archive: %w", err)
	}

	entries := make([]archiveEntry, 0, len(reader.File))
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open zip entry %s: %w", file.Name, err)
		}
		payload, readErr := io.ReadAll(rc)
		_ = rc.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read zip entry %s: %w", file.Name, readErr)
		}
		entries = append(entries, archiveEntry{
			Name:        file.Name,
			ContentType: http.DetectContentType(payload),
			Body:        payload,
		})
	}
	return entries, nil
}

func readTarGzEntries(body []byte) ([]archiveEntry, error) {
	gr, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("open gzip stream: %w", err)
	}
	defer gr.Close()
	return readTarEntries(gr)
}

func readTarEntries(reader io.Reader) ([]archiveEntry, error) {
	tr := tar.NewReader(reader)
	var entries []archiveEntry

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar entry: %w", err)
		}
		if header.Typeflag == tar.TypeDir {
			continue
		}

		payload, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("read tar payload %s: %w", header.Name, err)
		}
		entries = append(entries, archiveEntry{
			Name:        header.Name,
			ContentType: http.DetectContentType(payload),
			Body:        payload,
		})
	}
	return entries, nil
}

func readGzipEntry(name string, body []byte) ([]archiveEntry, error) {
	gr, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("open gzip file: %w", err)
	}
	defer gr.Close()

	payload, err := io.ReadAll(gr)
	if err != nil {
		return nil, fmt.Errorf("read gzip payload: %w", err)
	}

	entryName := strings.TrimSpace(gr.Name)
	if entryName == "" {
		entryName = trimArchiveExtension(filepath.Base(name))
	}
	if entryName == "" {
		entryName = "payload"
	}

	return []archiveEntry{{
		Name:        entryName,
		ContentType: http.DetectContentType(payload),
		Body:        payload,
	}}, nil
}

func isArchiveFile(name, contentType string, body []byte) bool {
	lowerName := strings.ToLower(strings.TrimSpace(name))
	lowerType := strings.ToLower(strings.TrimSpace(contentType))
	detected := strings.ToLower(http.DetectContentType(body))

	switch {
	case strings.HasSuffix(lowerName, ".zip"),
		strings.HasSuffix(lowerName, ".tar"),
		strings.HasSuffix(lowerName, ".tar.gz"),
		strings.HasSuffix(lowerName, ".tgz"),
		strings.HasSuffix(lowerName, ".gz"),
		strings.Contains(lowerType, "zip"),
		strings.Contains(lowerType, "gzip"),
		strings.Contains(lowerType, "x-tar"),
		strings.Contains(detected, "zip"),
		strings.Contains(detected, "gzip"):
		return true
	default:
		return false
	}
}

func isTextLike(name, contentType string, body []byte) bool {
	lowerType := strings.ToLower(strings.TrimSpace(contentType))
	if strings.HasPrefix(lowerType, "text/") {
		return true
	}

	switch strings.ToLower(filepath.Ext(name)) {
	case ".txt", ".log", ".json", ".jsonl", ".xml", ".yaml", ".yml", ".ini", ".cfg", ".conf", ".csv", ".tsv", ".md", ".sql", ".ps1", ".bat", ".cmd", ".sh", ".py", ".rb", ".go", ".java", ".js", ".ts", ".swift", ".kt", ".properties":
		return true
	}

	if bytes.IndexByte(body, 0) >= 0 {
		return false
	}
	if utf8.Valid(body) {
		return true
	}

	detected := strings.ToLower(http.DetectContentType(body))
	return strings.Contains(detected, "json") || strings.Contains(detected, "xml") || strings.Contains(detected, "text/plain")
}

func prefixedSafeName(index int, name string) string {
	base := sanitizeFSName(filepath.Base(strings.TrimSpace(name)))
	if base == "" {
		base = "attachment"
	}
	return fmt.Sprintf("%03d-%s", index, base)
}

func sanitizeArchiveEntryPath(name string) string {
	cleaned := path.Clean(strings.ReplaceAll(strings.TrimSpace(name), "\\", "/"))
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "." || cleaned == "" {
		return ""
	}

	rawParts := strings.Split(cleaned, "/")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		if part == "" || part == "." || part == ".." {
			continue
		}
		if sanitized := sanitizeFSName(part); sanitized != "" {
			parts = append(parts, sanitized)
		}
	}
	return strings.Join(parts, "/")
}

func sanitizeFSName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			if b.Len() == 0 || strings.HasSuffix(b.String(), "-") {
				continue
			}
			b.WriteByte('-')
		}
	}

	safe := strings.Trim(b.String(), ".-")
	if safe == "" {
		return "item"
	}
	return safe
}

func trimArchiveExtension(name string) string {
	lowerName := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lowerName, ".tar.gz"):
		return strings.TrimSuffix(name, name[len(name)-7:])
	case strings.HasSuffix(lowerName, ".tgz"):
		return strings.TrimSuffix(name, name[len(name)-4:])
	case strings.HasSuffix(lowerName, ".zip"):
		return strings.TrimSuffix(name, name[len(name)-4:])
	case strings.HasSuffix(lowerName, ".tar"):
		return strings.TrimSuffix(name, name[len(name)-4:])
	case strings.HasSuffix(lowerName, ".gz"):
		return strings.TrimSuffix(name, name[len(name)-3:])
	default:
		ext := filepath.Ext(name)
		return strings.TrimSuffix(name, ext)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func toString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return strconv.FormatInt(int64(typed), 10)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case json.Number:
		return typed.String()
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func toBool(value any) bool {
	typed, ok := value.(bool)
	return ok && typed
}

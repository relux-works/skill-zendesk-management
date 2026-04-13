package main

import (
	"path/filepath"
	"testing"
)

func TestResolveDestinationPathDefaultsToManagedTempDir(t *testing.T) {
	got := resolveDestinationPath("trace.log", "", "", "")
	want := filepath.Join(defaultAttachmentDownloadDir(), "trace.log")
	if got != want {
		t.Fatalf("resolveDestinationPath() = %q, want %q", got, want)
	}
}

func TestResolveDestinationPathUsesExistingDirectoryDestination(t *testing.T) {
	existingDir := t.TempDir()
	got := resolveDestinationPath("trace.log", existingDir, "", "")
	want := filepath.Join(existingDir, "trace.log")
	if got != want {
		t.Fatalf("resolveDestinationPath(existing dir) = %q, want %q", got, want)
	}
}

func TestResolveDestinationPathUsesDirectorySuffixDestination(t *testing.T) {
	got := resolveDestinationPath("trace.log", filepath.Join("artifacts", "logs")+string(filepath.Separator), "", "")
	want := filepath.Join("artifacts", "logs", "trace.log")
	if got != want {
		t.Fatalf("resolveDestinationPath(dir suffix) = %q, want %q", got, want)
	}
}

func TestResolveDestinationPathPreservesExplicitDirOverride(t *testing.T) {
	got := resolveDestinationPath("trace.log", "", "", filepath.Join("custom", "downloads"))
	want := filepath.Join("custom", "downloads", "trace.log")
	if got != want {
		t.Fatalf("resolveDestinationPath(explicit dir) = %q, want %q", got, want)
	}
}

package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gastownhall/offstage/internal/store"
)

// TestCloneRejectsExistingStore verifies that Clone returns an error when the
// target directory already contains files (second-init guard).
func TestCloneRejectsExistingStore(t *testing.T) {
	tmp := t.TempDir()
	storeDir := filepath.Join(tmp, "store")

	// Pre-populate the directory to simulate an existing clone.
	if err := os.MkdirAll(storeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(storeDir, "sentinel"), []byte("exists"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := store.Clone("git@github.com:example/store.git", storeDir)
	if err == nil {
		t.Fatal("expected error for existing store, got nil")
	}
}

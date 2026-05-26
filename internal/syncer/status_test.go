package syncer_test

import (
	"errors"
	"testing"

	"github.com/claytercek/offstage/internal/syncer"
)

// TestStatus_Clean verifies that Status returns nil and prints "everything in sync"
// when local and store content are identical. Uses Push to set up the store so
// only tracked files are present (mirroring real usage after a push).
func TestStatus_Clean(t *testing.T) {
	content := "# Context\n"

	remoteDir := setupBareRemote(t)
	s := setupLocalStore(t, remoteDir)

	projectDir := t.TempDir()
	writeFile(t, projectDir, "CONTEXT.md", content)

	if err := syncer.Push(s, projectDir, []string{"CONTEXT.md"}, nil, "test-project", "main", false); err != nil {
		t.Fatalf("Push: %v", err)
	}

	err := syncer.Status(s, projectDir, []string{"CONTEXT.md"}, nil, "test-project/main")
	if err != nil {
		t.Errorf("expected nil (clean), got: %v", err)
	}
}

// TestStatus_Modified verifies that Status returns ErrHasDiff when a tracked
// file has been modified locally.
func TestStatus_Modified(t *testing.T) {
	storeBranch := "test-project/main"

	remoteDir := setupBareRemote(t)
	s := setupLocalStore(t, remoteDir)
	seedStoreBranch(t, s, storeBranch, map[string]string{
		"CONTEXT.md": "# Context\noriginal\n",
	})

	projectDir := t.TempDir()
	writeFile(t, projectDir, "CONTEXT.md", "# Context\nmodified\n")

	err := syncer.Status(s, projectDir, []string{"CONTEXT.md"}, nil, storeBranch)
	if !errors.Is(err, syncer.ErrHasDiff) {
		t.Errorf("expected ErrHasDiff for modified file, got: %v", err)
	}
}

// TestStatus_NewFile verifies that Status returns ErrHasDiff when a tracked
// file exists locally but not in the store.
func TestStatus_NewFile(t *testing.T) {
	storeBranch := "test-project/main"

	remoteDir := setupBareRemote(t)
	s := setupLocalStore(t, remoteDir)
	seedStoreBranch(t, s, storeBranch, map[string]string{
		"CONTEXT.md": "# Context\n",
	})

	projectDir := t.TempDir()
	writeFile(t, projectDir, "CONTEXT.md", "# Context\n")
	writeFile(t, projectDir, "AGENTS.md", "# Agents\nnew\n")

	err := syncer.Status(s, projectDir, []string{"CONTEXT.md", "AGENTS.md"}, nil, storeBranch)
	if !errors.Is(err, syncer.ErrHasDiff) {
		t.Errorf("expected ErrHasDiff for new file, got: %v", err)
	}
}

// TestStatus_Deleted verifies that Status returns ErrHasDiff when the store
// contains a file no longer in the tracked set (would be deleted by push).
func TestStatus_Deleted(t *testing.T) {
	storeBranch := "test-project/main"

	remoteDir := setupBareRemote(t)
	s := setupLocalStore(t, remoteDir)
	seedStoreBranch(t, s, storeBranch, map[string]string{
		"CONTEXT.md": "# Context\n",
		"AGENTS.md":  "# Agents\n",
	})

	projectDir := t.TempDir()
	writeFile(t, projectDir, "CONTEXT.md", "# Context\n")

	// Only tracking CONTEXT.md — AGENTS.md is in store but not tracked locally.
	err := syncer.Status(s, projectDir, []string{"CONTEXT.md"}, nil, storeBranch)
	if !errors.Is(err, syncer.ErrHasDiff) {
		t.Errorf("expected ErrHasDiff for deleted file, got: %v", err)
	}
}

// TestStatus_NoBranch verifies that Status returns ErrHasDiff (all files "new")
// when the store branch doesn't exist yet.
func TestStatus_NoBranch(t *testing.T) {
	remoteDir := setupBareRemote(t)
	s := setupLocalStore(t, remoteDir)

	projectDir := t.TempDir()
	writeFile(t, projectDir, "CONTEXT.md", "# Context\n")

	err := syncer.Status(s, projectDir, []string{"CONTEXT.md"}, nil, "test-project/main")
	if !errors.Is(err, syncer.ErrHasDiff) {
		t.Errorf("expected ErrHasDiff when branch missing, got: %v", err)
	}
}

// TestStatus_NoBranchNoFiles verifies that Status returns nil when there is
// no store branch and no tracked files.
func TestStatus_NoBranchNoFiles(t *testing.T) {
	remoteDir := setupBareRemote(t)
	s := setupLocalStore(t, remoteDir)
	projectDir := t.TempDir()

	err := syncer.Status(s, projectDir, []string{"CONTEXT.md"}, nil, "test-project/main")
	if err != nil {
		t.Errorf("expected nil (no branch, no files), got: %v", err)
	}
}

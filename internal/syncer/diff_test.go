package syncer_test

import (
	"errors"
	"testing"

	"github.com/gastownhall/offstage/internal/syncer"
)

// TestDiffLocal_HasDiff verifies that DiffLocal returns ErrHasDiff when
// local and store content differ.
func TestDiffLocal_HasDiff(t *testing.T) {
	storeBranch := "test-project/main"
	contentA := "# Context\ncontent A\n"
	contentB := "# Context\ncontent B\n"

	remoteDir := setupBareRemote(t)
	s := setupLocalStore(t, remoteDir)
	seedStoreBranch(t, s, storeBranch, map[string]string{
		"CONTEXT.md": contentA,
	})

	// Set up project dir with content B (different from store's content A).
	projectDir := t.TempDir()
	writeFile(t, projectDir, "CONTEXT.md", contentB)

	include := []string{"CONTEXT.md"}
	exclude := []string{}

	err := syncer.DiffLocal(s, projectDir, include, exclude, storeBranch)
	if err == nil {
		t.Fatal("expected ErrHasDiff, got nil")
	}
	if !errors.Is(err, syncer.ErrHasDiff) {
		t.Errorf("expected ErrHasDiff, got: %v", err)
	}
}

// TestDiffLocal_NoDiff verifies that DiffLocal returns nil when local and
// store content are identical.
func TestDiffLocal_NoDiff(t *testing.T) {
	storeBranch := "test-project/main"
	contentA := "# Context\ncontent A\n"

	remoteDir := setupBareRemote(t)
	s := setupLocalStore(t, remoteDir)
	seedStoreBranch(t, s, storeBranch, map[string]string{
		"CONTEXT.md": contentA,
	})

	// Set up project dir with same content as store.
	projectDir := t.TempDir()
	writeFile(t, projectDir, "CONTEXT.md", contentA)

	include := []string{"CONTEXT.md"}
	exclude := []string{}

	err := syncer.DiffLocal(s, projectDir, include, exclude, storeBranch)
	if err != nil {
		t.Errorf("expected nil (no diff), got: %v", err)
	}
}

// TestDiffLocal_BranchNotFound verifies that DiffLocal returns a descriptive
// error when the store branch doesn't exist.
func TestDiffLocal_BranchNotFound(t *testing.T) {
	remoteDir := setupBareRemote(t)
	s := setupLocalStore(t, remoteDir)

	projectDir := t.TempDir()
	writeFile(t, projectDir, "CONTEXT.md", "some content\n")

	include := []string{"CONTEXT.md"}
	exclude := []string{}

	err := syncer.DiffLocal(s, projectDir, include, exclude, "nonexistent-project/main")
	if err == nil {
		t.Fatal("expected error for missing branch, got nil")
	}
	if errors.Is(err, syncer.ErrHasDiff) {
		t.Errorf("expected descriptive error, not ErrHasDiff")
	}
}

// TestDiffBranches_HasDiff verifies that DiffBranches returns ErrHasDiff when
// two store branches have different content.
func TestDiffBranches_HasDiff(t *testing.T) {
	remoteDir := setupBareRemote(t)
	s := setupLocalStore(t, remoteDir)

	branchA := "test-project/branch-a"
	branchB := "test-project/branch-b"

	seedStoreBranch(t, s, branchA, map[string]string{
		"CONTEXT.md": "# Context\ncontent for branch A\n",
	})
	seedStoreBranch(t, s, branchB, map[string]string{
		"CONTEXT.md": "# Context\ncontent for branch B\n",
	})

	err := syncer.DiffBranches(s, branchA, branchB)
	if err == nil {
		t.Fatal("expected ErrHasDiff, got nil")
	}
	if !errors.Is(err, syncer.ErrHasDiff) {
		t.Errorf("expected ErrHasDiff, got: %v", err)
	}
}

// TestDiffBranches_BranchNotFound verifies that DiffBranches returns a
// descriptive error when a branch doesn't exist.
func TestDiffBranches_BranchNotFound(t *testing.T) {
	remoteDir := setupBareRemote(t)
	s := setupLocalStore(t, remoteDir)

	seedStoreBranch(t, s, "test-project/main", map[string]string{
		"CONTEXT.md": "# Context\n",
	})

	// Target branch doesn't exist.
	err := syncer.DiffBranches(s, "test-project/main", "test-project/nonexistent")
	if err == nil {
		t.Fatal("expected error for missing target branch, got nil")
	}
	if errors.Is(err, syncer.ErrHasDiff) {
		t.Errorf("expected descriptive error for missing branch, not ErrHasDiff")
	}

	// Current branch doesn't exist.
	err = syncer.DiffBranches(s, "test-project/nonexistent", "test-project/main")
	if err == nil {
		t.Fatal("expected error for missing current branch, got nil")
	}
	if errors.Is(err, syncer.ErrHasDiff) {
		t.Errorf("expected descriptive error for missing branch, not ErrHasDiff")
	}
}

// TestDiffBranches_NoDiff verifies that DiffBranches returns nil when two
// branches have identical content.
func TestDiffBranches_NoDiff(t *testing.T) {
	remoteDir := setupBareRemote(t)
	s := setupLocalStore(t, remoteDir)

	storeBranch := "test-project/main"

	seedStoreBranch(t, s, storeBranch, map[string]string{
		"CONTEXT.md": "# Context\nidentical content\n",
	})

	// Create a second branch pointing at the same commit.
	if err := s.Checkout(storeBranch); err != nil {
		t.Fatalf("checkout %s: %v", storeBranch, err)
	}
	if err := s.Exec("checkout", "-b", "test-project/feature"); err != nil {
		t.Fatalf("create feature branch: %v", err)
	}
	if err := s.Checkout(storeBranch); err != nil {
		t.Fatalf("return to main: %v", err)
	}

	err := syncer.DiffBranches(s, storeBranch, "test-project/feature")
	if err != nil {
		t.Errorf("expected nil (no diff), got: %v", err)
	}
}

// TestDiffLocal_FileOnlyInLocal verifies that DiffLocal detects a file that
// exists locally but not in the store.
func TestDiffLocal_FileOnlyInLocal(t *testing.T) {
	storeBranch := "test-project/main"

	remoteDir := setupBareRemote(t)
	s := setupLocalStore(t, remoteDir)
	seedStoreBranch(t, s, storeBranch, map[string]string{
		"CONTEXT.md": "# Context\n",
	})

	// Local project has an extra file not in the store.
	projectDir := t.TempDir()
	writeFile(t, projectDir, "CONTEXT.md", "# Context\n")
	writeFile(t, projectDir, "AGENTS.md", "# Agents\nnew file\n")

	include := []string{"CONTEXT.md", "AGENTS.md"}
	exclude := []string{}

	err := syncer.DiffLocal(s, projectDir, include, exclude, storeBranch)
	if err == nil {
		t.Fatal("expected ErrHasDiff (file only in local), got nil")
	}
	if !errors.Is(err, syncer.ErrHasDiff) {
		t.Errorf("expected ErrHasDiff, got: %v", err)
	}
}

// TestDiffLocal_IgnoresExcluded verifies that excluded files are not compared.
func TestDiffLocal_IgnoresExcluded(t *testing.T) {
	storeBranch := "test-project/main"
	contentA := "# Context\ncontent A\n"

	remoteDir := setupBareRemote(t)
	s := setupLocalStore(t, remoteDir)
	seedStoreBranch(t, s, storeBranch, map[string]string{
		"CONTEXT.md": contentA,
	})

	projectDir := t.TempDir()
	writeFile(t, projectDir, "CONTEXT.md", contentA)
	writeFile(t, projectDir, "secret.md", "sensitive stuff\n")

	// secret.md is excluded so no diff expected even though it's not in store.
	include := []string{"CONTEXT.md", "secret.md"}
	exclude := []string{"secret.md"}

	err := syncer.DiffLocal(s, projectDir, include, exclude, storeBranch)
	if err != nil {
		t.Errorf("expected nil (excluded file ignored), got: %v", err)
	}
}

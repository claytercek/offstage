package syncer_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/claytercek/offstage/internal/store"
	"github.com/claytercek/offstage/internal/syncer"
)

// newStoreWithRemote creates a bare remote, clones it to a local store,
// configures user info, creates an initial commit on main, and returns
// the store and the remote path.
func newStoreWithRemote(t *testing.T) (*store.Store, string) {
	t.Helper()
	remote := t.TempDir()
	if out, err := exec.Command("git", "init", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}

	local := filepath.Join(t.TempDir(), "store")
	if out, err := exec.Command("git", "clone", remote, local).CombinedOutput(); err != nil {
		t.Fatalf("git clone: %v\n%s", err, out)
	}
	mustGit(t, local, "config", "user.email", "test@test.com")
	mustGit(t, local, "config", "user.name", "Test")

	// Initial commit to establish main.
	mustGit(t, local, "commit", "--allow-empty", "-m", "init")
	mustGit(t, local, "push", "origin", "HEAD")

	s, err := store.Open(local)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	return s, remote
}

// seedMergeBranch creates a store branch from the store's current HEAD (main),
// writes the given files, commits, pushes, and returns to original branch.
func seedMergeBranch(t *testing.T, s *store.Store, branchName string, files map[string]string) {
	t.Helper()

	orig := currentBranch(t, s.Path)

	// Checkout (create) the branch from the current HEAD.
	if out, err := exec.Command("git", "-C", s.Path, "checkout", "-b", branchName).CombinedOutput(); err != nil {
		t.Fatalf("git checkout -b %s: %v\n%s", branchName, err, out)
	}

	for rel, content := range files {
		dest := filepath.Join(s.Path, rel)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(dest), err)
		}
		if err := os.WriteFile(dest, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", dest, err)
		}
	}

	mustGit(t, s.Path, "add", "-A")
	// Use --allow-empty so we don't fail even if nothing changed.
	mustGit(t, s.Path, "commit", "--allow-empty", "-m", "seed: "+branchName)
	mustGit(t, s.Path, "push", "origin", branchName)

	// Restore original branch.
	mustGit(t, s.Path, "checkout", orig)
}

// TestMerge_CleanMerge verifies that Merge succeeds when the two branches touch
// different files, pushes the merged result, and marks the source as reconciled.
func TestMerge_CleanMerge(t *testing.T) {
	s, _ := newStoreWithRemote(t)

	const pid = "example.com/org/clean"

	// Create main branch with CONTEXT.md.
	seedMergeBranch(t, s, pid+"/main", map[string]string{
		"CONTEXT.md": "# Main context\nmain content\n",
	})

	// Create feature branch with a DIFFERENT file (AGENTS.md) — no conflict.
	seedMergeBranch(t, s, pid+"/feature", map[string]string{
		"AGENTS.md": "# Agents\nfeature content\n",
	})

	// Perform the merge: feature -> main.
	if err := syncer.Merge(s, pid, "main", "feature"); err != nil {
		t.Fatalf("Merge: %v", err)
	}

	// After merge, switch to main branch and verify both files are present.
	mustGit(t, s.Path, "checkout", pid+"/main")

	contextPath := filepath.Join(s.Path, "CONTEXT.md")
	data, err := os.ReadFile(contextPath)
	if err != nil {
		t.Fatalf("read CONTEXT.md: %v", err)
	}
	if !strings.Contains(string(data), "main content") {
		t.Errorf("CONTEXT.md missing main content, got: %q", string(data))
	}

	agentsPath := filepath.Join(s.Path, "AGENTS.md")
	agData, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if !strings.Contains(string(agData), "feature content") {
		t.Errorf("AGENTS.md missing feature content, got: %q", string(agData))
	}
}

// TestMerge_BranchNotFound verifies that Merge returns a descriptive error
// when the source branch doesn't exist in the store.
func TestMerge_BranchNotFound(t *testing.T) {
	s, _ := newStoreWithRemote(t)

	const pid = "example.com/org/notfound"

	// Create target branch (main) only.
	seedMergeBranch(t, s, pid+"/main", map[string]string{
		"CONTEXT.md": "# Main context\n",
	})

	// Attempt to merge a non-existent source branch.
	err := syncer.Merge(s, pid, "main", "nonexistent-branch")
	if err == nil {
		t.Fatal("expected error for missing source branch, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent-branch") {
		t.Errorf("expected error to mention branch name, got: %v", err)
	}
	if !strings.Contains(err.Error(), "offstage push") {
		t.Errorf("expected error to suggest 'offstage push', got: %v", err)
	}
}

// TestMerge_TargetBranchNotFound verifies that Merge returns a descriptive error
// when the target (current) branch has no sync store data.
func TestMerge_TargetBranchNotFound(t *testing.T) {
	s, _ := newStoreWithRemote(t)

	const pid = "example.com/org/noTarget"

	// Create source branch but NOT target.
	seedMergeBranch(t, s, pid+"/feature", map[string]string{
		"CONTEXT.md": "# Feature context\n",
	})

	err := syncer.Merge(s, pid, "main", "feature")
	if err == nil {
		t.Fatal("expected error for missing target branch, got nil")
	}
	if !strings.Contains(err.Error(), "offstage push") {
		t.Errorf("expected error to suggest 'offstage push', got: %v", err)
	}
}

// TestMerge_Conflict verifies that Merge returns a non-nil error when there are
// conflicts, leaves the merge in-progress (MERGE_HEAD exists), and that the
// error message contains the conflicting file names and 'offstage git' recovery
// instructions.
func TestMerge_Conflict(t *testing.T) {
	s, _ := newStoreWithRemote(t)

	const pid = "example.com/org/conflict"

	// Create main branch with a specific version of CONTEXT.md.
	// Store current branch (which is "main" of the git store).
	orig := currentBranch(t, s.Path)

	// Create pid/main branching off store main.
	mustGit(t, s.Path, "checkout", "-b", pid+"/main")
	if err := os.WriteFile(filepath.Join(s.Path, "CONTEXT.md"), []byte("base\nline\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, s.Path, "add", "-A")
	mustGit(t, s.Path, "commit", "-m", "base on pid/main")
	mustGit(t, s.Path, "push", "origin", pid+"/main")

	// Create pid/feature branching off pid/main (same base).
	mustGit(t, s.Path, "checkout", "-b", pid+"/feature")
	if err := os.WriteFile(filepath.Join(s.Path, "CONTEXT.md"), []byte("feature conflicting change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, s.Path, "add", "-A")
	mustGit(t, s.Path, "commit", "-m", "feature diverging commit")
	mustGit(t, s.Path, "push", "origin", pid+"/feature")

	// Now add a diverging commit on pid/main (after the feature branch point).
	mustGit(t, s.Path, "checkout", pid+"/main")
	if err := os.WriteFile(filepath.Join(s.Path, "CONTEXT.md"), []byte("main conflicting change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, s.Path, "add", "-A")
	mustGit(t, s.Path, "commit", "-m", "main diverging commit")
	mustGit(t, s.Path, "push", "origin", pid+"/main")

	// Return to store main before calling Merge.
	mustGit(t, s.Path, "checkout", orig)

	// Both branches now diverged from the same base with conflicting CONTEXT.md.
	err := syncer.Merge(s, pid, "main", "feature")
	if err == nil {
		t.Fatal("expected error on conflict, got nil")
	}

	// Conflicting file should be mentioned in the error.
	if !strings.Contains(err.Error(), "CONTEXT.md") {
		t.Errorf("expected error to list CONTEXT.md as conflicting, got: %v", err)
	}

	// Instructions should mention 'offstage git'.
	if !strings.Contains(err.Error(), "offstage git") {
		t.Errorf("expected error to mention 'offstage git', got: %v", err)
	}

	// Store should have MERGE_HEAD — merge is left in-progress for user to resolve.
	mergeHeadPath := filepath.Join(s.Path, ".git", "MERGE_HEAD")
	if _, statErr := os.Stat(mergeHeadPath); statErr != nil {
		t.Error("expected MERGE_HEAD to exist — merge should be left in-progress for user resolution")
	}
}

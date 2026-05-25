package syncer_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gastownhall/offstage/internal/store"
	"github.com/gastownhall/offstage/internal/syncer"
)

// mustRun is a helper that runs a command and fails the test if it errors.
func mustRun(t *testing.T, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("run %s %v: %v\n%s", name, args, err, out)
	}
}

// initBareRemoteAndStore creates a bare remote, clones it to a local store,
// and returns the store and the remote path.
func initBareRemoteAndStore(t *testing.T) (*store.Store, string) {
	t.Helper()
	remote := t.TempDir()
	mustRun(t, "git", "init", "--bare", remote)

	storeDir := t.TempDir()
	mustRun(t, "git", "clone", remote, storeDir)
	mustRun(t, "git", "-C", storeDir, "config", "user.email", "test@test.com")
	mustRun(t, "git", "-C", storeDir, "config", "user.name", "Test")

	// Create an initial commit so the default branch exists.
	if err := os.WriteFile(filepath.Join(storeDir, "README"), []byte("init"), 0o600); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "git", "-C", storeDir, "add", ".")
	mustRun(t, "git", "-C", storeDir, "commit", "-m", "initial commit")
	mustRun(t, "git", "-C", storeDir, "push", "origin", "HEAD")

	s, err := store.Open(storeDir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	return s, remote
}

// seedStoreBranch creates a store branch with the given files and pushes it to
// origin. Each entry in files maps relative path -> content.
func seedStoreBranch(t *testing.T, s *store.Store, storeBranch string, files map[string]string) {
	t.Helper()

	// Get current (default) branch to return to after seeding.
	defaultBranch := currentBranch(t, s.Path)

	// Create the branch.
	if err := s.Exec("checkout", "-b", storeBranch); err != nil {
		t.Fatalf("create store branch %s: %v", storeBranch, err)
	}

	for rel, content := range files {
		dest := filepath.Join(s.Path, rel)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dest, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	mustRun(t, "git", "-C", s.Path, "add", "-A")
	mustRun(t, "git", "-C", s.Path, "commit", "-m", "seed files for pull test")

	// Push the branch to origin.
	if err := s.Exec("push", "origin", storeBranch); err != nil {
		t.Fatalf("push store branch %s: %v", storeBranch, err)
	}

	// Return to the default branch.
	if err := s.Checkout(defaultBranch); err != nil {
		t.Fatalf("checkout default branch: %v", err)
	}
}

// currentBranch returns the current branch in the given git directory.
func currentBranch(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "symbolic-ref", "--short", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("get current branch: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// TestPullCleanBranch verifies that Pull copies files from the store branch
// to the project directory when the branch exists and is clean.
func TestPullCleanBranch(t *testing.T) {
	s, _ := initBareRemoteAndStore(t)

	projectID := "example.com/org/project"
	branchName := "main"
	storeBranch := projectID + "/" + branchName

	// Seed the store branch with test files.
	seedStoreBranch(t, s, storeBranch, map[string]string{
		"CONTEXT.md":        "# Project context",
		"subdir/AGENTS.md":  "# Agents",
	})

	// Pull into a fresh project dir.
	projectDir := t.TempDir()
	if err := syncer.Pull(s, projectDir, projectID, branchName, false); err != nil {
		t.Fatalf("Pull: %v", err)
	}

	// Verify files were copied.
	checkFile(t, filepath.Join(projectDir, "CONTEXT.md"), "# Project context")
	checkFile(t, filepath.Join(projectDir, "subdir", "AGENTS.md"), "# Agents")
}

// TestPullBranchNotFound verifies that Pull returns ErrBranchNotFound when the
// store branch doesn't exist in either local or remote.
func TestPullBranchNotFound(t *testing.T) {
	s, _ := initBareRemoteAndStore(t)

	projectDir := t.TempDir()
	err := syncer.Pull(s, projectDir, "example.com/nonexistent", "main", false)
	if err == nil {
		t.Fatal("expected ErrBranchNotFound, got nil")
	}
	if !errors.Is(err, syncer.ErrBranchNotFound) {
		t.Errorf("expected ErrBranchNotFound, got: %v", err)
	}
}

// TestPullDryRun verifies that Pull with dryRun=true lists files but does NOT
// write anything to the project directory.
func TestPullDryRun(t *testing.T) {
	s, _ := initBareRemoteAndStore(t)

	projectID := "example.com/org/dryrun"
	branchName := "feature"
	storeBranch := projectID + "/" + branchName

	seedStoreBranch(t, s, storeBranch, map[string]string{
		"CONTEXT.md": "# Dry run context",
	})

	projectDir := t.TempDir()
	if err := syncer.Pull(s, projectDir, projectID, branchName, true); err != nil {
		t.Fatalf("Pull dry-run: %v", err)
	}

	// Project directory should be empty (no files written).
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("dry-run wrote files to project dir: %v", entries)
	}
}

// TestPullDiverged verifies that Pull returns ErrDiverged when both local and
// remote store branches have unique commits.
func TestPullDiverged(t *testing.T) {
	s, remote := initBareRemoteAndStore(t)

	projectID := "example.com/org/diverged"
	branchName := "main"
	storeBranch := projectID + "/" + branchName

	// Seed the store branch with an initial file and push.
	seedStoreBranch(t, s, storeBranch, map[string]string{
		"initial.txt": "initial",
	})

	// Create a second clone to simulate a diverged remote.
	clone2 := t.TempDir()
	mustRun(t, "git", "clone", remote, clone2)
	mustRun(t, "git", "-C", clone2, "config", "user.email", "test@test.com")
	mustRun(t, "git", "-C", clone2, "config", "user.name", "Test")

	// Check out the store branch in clone2 and make a commit.
	mustRun(t, "git", "-C", clone2, "checkout", "-b", storeBranch, "origin/"+storeBranch)
	if err := os.WriteFile(filepath.Join(clone2, "remote-change.txt"), []byte("remote"), 0o600); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "git", "-C", clone2, "add", "-A")
	mustRun(t, "git", "-C", clone2, "commit", "-m", "remote diverging commit")
	mustRun(t, "git", "-C", clone2, "push", "origin", storeBranch)

	// Now make a local commit on s (after fetching).
	if err := s.Fetch(); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	// Checkout the store branch locally.
	if err := s.Exec("checkout", storeBranch); err != nil {
		t.Fatalf("checkout storeBranch in s: %v", err)
	}
	// Make a local diverging commit (without pulling).
	if err := os.WriteFile(filepath.Join(s.Path, "local-change.txt"), []byte("local"), 0o600); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "git", "-C", s.Path, "add", "-A")
	mustRun(t, "git", "-C", s.Path, "commit", "-m", "local diverging commit")

	// Return to default branch.
	defaultBranch := currentBranch(t, s.Path)
	_ = defaultBranch
	// Note: we're now on storeBranch; Pull will fetch and detect divergence.

	projectDir := t.TempDir()
	err := syncer.Pull(s, projectDir, projectID, branchName, false)
	if err == nil {
		t.Fatal("expected ErrDiverged, got nil")
	}
	if !errors.Is(err, syncer.ErrDiverged) {
		t.Errorf("expected ErrDiverged, got: %v", err)
	}
}

// checkFile asserts that the file at path exists and has the expected content.
func checkFile(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(data) != want {
		t.Errorf("file %s: got %q, want %q", path, string(data), want)
	}
}

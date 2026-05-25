package store_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// initTestRepo creates a git repo in dir with an initial commit and sets
// test identity. Returns the store.
func initTestRepo(t *testing.T, dir string) *store.Store {
	t.Helper()
	mustRun(t, "git", "init", dir)
	mustRun(t, "git", "-C", dir, "config", "user.email", "test@test.com")
	mustRun(t, "git", "-C", dir, "config", "user.name", "Test")
	// Create an initial commit so HEAD resolves.
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("init"), 0o600); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "git", "-C", dir, "add", ".")
	mustRun(t, "git", "-C", dir, "commit", "-m", "initial commit")

	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	return s
}

// initBareRemote creates a bare git repo and returns a local clone of it (as a
// *store.Store) together with the bare repo path.
func initBareRemote(t *testing.T) (s *store.Store, remotePath string) {
	t.Helper()
	remote := t.TempDir()
	mustRun(t, "git", "init", "--bare", remote)

	local := t.TempDir()
	mustRun(t, "git", "clone", remote, local)
	mustRun(t, "git", "-C", local, "config", "user.email", "test@test.com")
	mustRun(t, "git", "-C", local, "config", "user.name", "Test")

	// Create an initial commit so the default branch exists on the remote.
	if err := os.WriteFile(filepath.Join(local, "README"), []byte("init"), 0o600); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "git", "-C", local, "add", ".")
	mustRun(t, "git", "-C", local, "commit", "-m", "initial commit")
	mustRun(t, "git", "-C", local, "push", "origin", "HEAD")

	opened, err := store.Open(local)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	return opened, remote
}

func mustRun(t *testing.T, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("run %s %v: %v\n%s", name, args, err, out)
	}
}

// TestOpenFailsOnNonGitDir verifies that Open returns an error for a plain
// (non-git) directory.
func TestOpenFailsOnNonGitDir(t *testing.T) {
	dir := t.TempDir()
	_, err := store.Open(dir)
	if err == nil {
		t.Fatal("expected error opening non-git dir, got nil")
	}
}

// TestOpenSucceedsOnGitRepo verifies that Open succeeds on a real git repo.
func TestOpenSucceedsOnGitRepo(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, "git", "init", dir)
	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Path != dir {
		t.Errorf("Path = %q, want %q", s.Path, dir)
	}
}

// TestExecOutputRunsGitLog verifies that ExecOutput can run git log and
// returns non-empty output.
func TestExecOutputRunsGitLog(t *testing.T) {
	dir := t.TempDir()
	s := initTestRepo(t, dir)

	out, err := s.ExecOutput("log", "--oneline")
	if err != nil {
		t.Fatalf("ExecOutput git log: %v", err)
	}
	if !strings.Contains(out, "initial commit") {
		t.Errorf("expected 'initial commit' in output, got: %q", out)
	}
}

// TestCreateBranchAndCheckout verifies that CreateBranch creates a branch and
// Checkout switches to it.
func TestCreateBranchAndCheckout(t *testing.T) {
	dir := t.TempDir()
	s := initTestRepo(t, dir)

	if err := s.CreateBranch("feature-x"); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}

	branch, err := s.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if branch != "feature-x" {
		t.Errorf("CurrentBranch = %q, want %q", branch, "feature-x")
	}

	// Switch back to the original branch.
	defaultBranch := defaultBranchName(t, dir)
	if err := s.Checkout(defaultBranch); err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	branch, err = s.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch after checkout: %v", err)
	}
	if branch != defaultBranch {
		t.Errorf("CurrentBranch = %q, want %q", branch, defaultBranch)
	}
}

// TestAddAndCommit verifies that Add + Commit create a real commit.
func TestAddAndCommit(t *testing.T) {
	dir := t.TempDir()
	s := initTestRepo(t, dir)

	// Create a new file to stage.
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := s.Add("hello.txt"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := s.Commit("add hello.txt"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Verify the commit shows up in the log.
	out, err := s.ExecOutput("log", "--oneline")
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if !strings.Contains(out, "add hello.txt") {
		t.Errorf("expected 'add hello.txt' in log, got: %q", out)
	}
}

// TestBranchExists verifies correct results for existing and non-existing
// branches.
func TestBranchExists(t *testing.T) {
	dir := t.TempDir()
	s := initTestRepo(t, dir)

	defaultBranch := defaultBranchName(t, dir)

	if !s.BranchExists(defaultBranch) {
		t.Errorf("BranchExists(%q) = false, want true", defaultBranch)
	}
	if s.BranchExists("nonexistent-branch") {
		t.Error("BranchExists(nonexistent-branch) = true, want false")
	}

	// Create a branch and verify it appears.
	if err := s.CreateBranch("my-branch"); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	if !s.BranchExists("my-branch") {
		t.Error("BranchExists(my-branch) = false after CreateBranch, want true")
	}
}

// TestPushToBareRemote verifies that Push sends commits to a bare remote.
func TestPushToBareRemote(t *testing.T) {
	s, remote := initBareRemote(t)

	// Make a new commit in the local clone.
	if err := os.WriteFile(filepath.Join(s.Path, "pushed.txt"), []byte("pushed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.Add("pushed.txt"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := s.Commit("push test commit"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if err := s.Push(); err != nil {
		t.Fatalf("Push: %v", err)
	}

	// Verify the commit is on the bare remote by checking the log there.
	cmd := exec.Command("git", "-C", remote, "log", "--oneline")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log on remote: %v", err)
	}
	if !strings.Contains(string(out), "push test commit") {
		t.Errorf("expected 'push test commit' in remote log, got: %q", string(out))
	}
}

// TestFetchAndPull verifies Fetch and Pull bring in remote changes.
func TestFetchAndPull(t *testing.T) {
	// Set up a bare remote and two clones.
	remote := t.TempDir()
	mustRun(t, "git", "init", "--bare", remote)

	clone1 := t.TempDir()
	mustRun(t, "git", "clone", remote, clone1)
	mustRun(t, "git", "-C", clone1, "config", "user.email", "test@test.com")
	mustRun(t, "git", "-C", clone1, "config", "user.name", "Test")

	// Initial commit from clone1.
	if err := os.WriteFile(filepath.Join(clone1, "init.txt"), []byte("init"), 0o600); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "git", "-C", clone1, "add", ".")
	mustRun(t, "git", "-C", clone1, "commit", "-m", "initial")
	mustRun(t, "git", "-C", clone1, "push", "origin", "HEAD")

	clone2 := t.TempDir()
	mustRun(t, "git", "clone", remote, clone2)
	mustRun(t, "git", "-C", clone2, "config", "user.email", "test@test.com")
	mustRun(t, "git", "-C", clone2, "config", "user.name", "Test")

	s2, err := store.Open(clone2)
	if err != nil {
		t.Fatalf("store.Open clone2: %v", err)
	}

	// Push a new commit from clone1.
	if err := os.WriteFile(filepath.Join(clone1, "new.txt"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "git", "-C", clone1, "add", ".")
	mustRun(t, "git", "-C", clone1, "commit", "-m", "new commit from clone1")
	mustRun(t, "git", "-C", clone1, "push", "origin", "HEAD")

	// Fetch from clone2, then pull.
	if err := s2.Fetch(); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if err := s2.Pull(); err != nil {
		t.Fatalf("Pull: %v", err)
	}

	// Verify the new commit is now in clone2.
	out, err := s2.ExecOutput("log", "--oneline")
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if !strings.Contains(out, "new commit from clone1") {
		t.Errorf("expected 'new commit from clone1' in log after pull, got: %q", out)
	}
}

// TestMergeBranchConflictReturnsError verifies that MergeBranch returns an
// error when there is a conflict.
func TestMergeBranchConflictReturnsError(t *testing.T) {
	dir := t.TempDir()
	s := initTestRepo(t, dir)
	defaultBranch := defaultBranchName(t, dir)

	// Create a feature branch with a conflicting change.
	if err := s.CreateBranch("feature-conflict"); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "conflict.txt"), []byte("feature version"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.Add("conflict.txt"); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit("feature change"); err != nil {
		t.Fatal(err)
	}

	// Switch back to default branch and make a conflicting change.
	if err := s.Checkout(defaultBranch); err != nil {
		t.Fatalf("Checkout default: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "conflict.txt"), []byte("main version"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.Add("conflict.txt"); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit("main change"); err != nil {
		t.Fatal(err)
	}

	// Merge should fail with conflict.
	err := s.MergeBranch("feature-conflict")
	if err == nil {
		t.Fatal("expected conflict error from MergeBranch, got nil")
	}
}

// defaultBranchName returns the current branch name for a fresh repo
// (works for both "main" and "master" depending on git config).
func defaultBranchName(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "symbolic-ref", "--short", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("get default branch: %v", err)
	}
	return strings.TrimSpace(string(out))
}

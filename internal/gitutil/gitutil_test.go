package gitutil_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/claytercek/offstage/internal/gitutil"
)

// initTestRepo creates a minimal git repo with an initial commit in dir.
func initTestRepo(t *testing.T, dir string) {
	t.Helper()
	mustRun := func(name string, args ...string) {
		t.Helper()
		cmd := exec.Command(name, args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("run %s %v: %v\n%s", name, args, err, out)
		}
	}
	mustRun("git", "init", "-b", "main", dir)
	mustRun("git", "-C", dir, "config", "user.email", "test@test.com")
	mustRun("git", "-C", dir, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("init"), 0o600); err != nil {
		t.Fatal(err)
	}
	mustRun("git", "-C", dir, "add", ".")
	mustRun("git", "-C", dir, "commit", "-m", "initial commit")
}

// TestBinReturnsValidPath verifies that Bin returns a non-empty path to git.
func TestBinReturnsValidPath(t *testing.T) {
	p := gitutil.Bin()
	if p == "" {
		t.Fatal("Bin() returned empty string")
	}
}

// TestOutputReturnsStdout verifies that Output captures stdout from git.
func TestOutputReturnsStdout(t *testing.T) {
	dir := t.TempDir()
	initTestRepo(t, dir)

	out, err := gitutil.Output(dir, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		t.Fatalf("Output: %v", err)
	}
	if strings.TrimSpace(out) == "" {
		t.Error("Output returned empty string for symbolic-ref")
	}
}

// TestOutputReturnsErrorOnBadCommand verifies that Output returns an error
// when the git command fails.
func TestOutputReturnsErrorOnBadCommand(t *testing.T) {
	dir := t.TempDir()
	initTestRepo(t, dir)

	_, err := gitutil.Output(dir, "nonexistent-git-subcommand-xyz")
	if err == nil {
		t.Fatal("expected error from invalid git command, got nil")
	}
}

// TestRunExecutesCommand verifies that Run can execute a git command
// successfully in a given directory.
func TestRunExecutesCommand(t *testing.T) {
	dir := t.TempDir()
	initTestRepo(t, dir)

	// git status should succeed in a git repo.
	err := gitutil.Run(dir, "status")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
}

// TestRunReturnsErrorOnFailure verifies that Run returns an error when
// the git command fails.
func TestRunReturnsErrorOnFailure(t *testing.T) {
	dir := t.TempDir()
	initTestRepo(t, dir)

	// Referencing a non-existent ref should fail.
	err := gitutil.Run(dir, "rev-parse", "--verify", "refs/heads/no-such-branch")
	if err == nil {
		t.Fatal("expected error from failing git command, got nil")
	}
}

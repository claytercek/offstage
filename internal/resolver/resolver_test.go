package resolver_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/claytercek/offstage/internal/resolver"
)

// initRepo creates a temporary git repository with an optional remote URL and
// returns its path.
func initRepo(t *testing.T, remoteURL string) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
	if remoteURL != "" {
		run("remote", "add", "origin", remoteURL)
	}
	return dir
}

func TestHTTPSRemoteNormalization(t *testing.T) {
	dir := initRepo(t, "https://github.com/clay/my-project.git")
	result, err := resolver.Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := "github.com/clay/my-project"
	if result.ProjectID != want {
		t.Errorf("ProjectID: got %q, want %q", result.ProjectID, want)
	}
}

func TestSSHRemoteNormalization(t *testing.T) {
	dir := initRepo(t, "git@github.com:clay/my-project.git")
	result, err := resolver.Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := "github.com/clay/my-project"
	if result.ProjectID != want {
		t.Errorf("ProjectID: got %q, want %q", result.ProjectID, want)
	}
}

func TestGitSuffixStripped(t *testing.T) {
	// Both HTTPS (with .git) and without .git should yield the same result.
	dirWith := initRepo(t, "https://github.com/clay/my-project.git")
	dirWithout := initRepo(t, "https://github.com/clay/my-project")

	r1, err := resolver.Resolve(dirWith)
	if err != nil {
		t.Fatalf("Resolve with .git: %v", err)
	}
	r2, err := resolver.Resolve(dirWithout)
	if err != nil {
		t.Fatalf("Resolve without .git: %v", err)
	}
	if r1.ProjectID != r2.ProjectID {
		t.Errorf("ProjectID mismatch: %q vs %q", r1.ProjectID, r2.ProjectID)
	}
}

func TestUppercaseLowercased(t *testing.T) {
	dir := initRepo(t, "https://GitHub.COM/Clay/My-Project.git")
	result, err := resolver.Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if strings.ToLower(result.ProjectID) != result.ProjectID {
		t.Errorf("ProjectID not lowercased: %q", result.ProjectID)
	}
	want := "github.com/clay/my-project"
	if result.ProjectID != want {
		t.Errorf("ProjectID: got %q, want %q", result.ProjectID, want)
	}
}

func TestWorktreeBranchDetection(t *testing.T) {
	// Create the main repo with a commit so we can add a worktree.
	dir := initRepo(t, "https://github.com/clay/my-project.git")

	// We need at least one commit to create a worktree.
	run := func(repoDir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	// Create an initial commit.
	run(dir, "commit", "--allow-empty", "-m", "initial commit")
	// Create a new branch.
	run(dir, "branch", "feature/worktree-test")

	// Create a worktree for the feature branch.
	wtDir := t.TempDir()
	run(dir, "worktree", "add", wtDir, "feature/worktree-test")

	result, err := resolver.Resolve(wtDir)
	if err != nil {
		t.Fatalf("Resolve in worktree: %v", err)
	}
	wantBranch := "feature/worktree-test"
	if result.BranchName != wantBranch {
		t.Errorf("BranchName in worktree: got %q, want %q", result.BranchName, wantBranch)
	}
	wantProject := "github.com/clay/my-project"
	if result.ProjectID != wantProject {
		t.Errorf("ProjectID in worktree: got %q, want %q", result.ProjectID, wantProject)
	}
}

func TestNameOverrideInOffstageRC(t *testing.T) {
	dir := initRepo(t, "https://github.com/clay/my-project.git")

	// Write a .offstagerc.toml with a name override.
	rcPath := filepath.Join(dir, ".offstagerc.toml")
	if err := os.WriteFile(rcPath, []byte(`name = "my-custom-project-id"`+"\n"), 0o644); err != nil {
		t.Fatalf("write .offstagerc.toml: %v", err)
	}

	result, err := resolver.Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := "my-custom-project-id"
	if result.ProjectID != want {
		t.Errorf("ProjectID: got %q, want %q (name override should take precedence)", result.ProjectID, want)
	}
}

func TestErrorNotInGitRepo(t *testing.T) {
	// A plain temp dir with no git init should yield a descriptive error.
	dir := t.TempDir()
	_, err := resolver.Resolve(dir)
	if err == nil {
		t.Fatal("expected an error for non-git directory, got nil")
	}
	if !strings.Contains(err.Error(), "git repository") {
		t.Errorf("error message should mention 'git repository', got: %v", err)
	}
}

func TestErrorNoRemote(t *testing.T) {
	// A git repo with no remote should return a descriptive error.
	dir := initRepo(t, "") // no remote URL
	_, err := resolver.Resolve(dir)
	if err == nil {
		t.Fatal("expected an error for repo with no remote, got nil")
	}
	if !strings.Contains(err.Error(), "origin") {
		t.Errorf("error message should mention 'origin', got: %v", err)
	}
}

package resolver_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/claytercek/offstage/internal/resolver"
)

func TestResolveBranchContextMain(t *testing.T) {
	dir := initRepo(t, "https://github.com/clay/my-project.git")

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("commit", "--allow-empty", "-m", "initial commit")

	got, err := resolver.ResolveBranchContext(dir)
	if err != nil {
		t.Fatalf("ResolveBranchContext: %v", err)
	}
	if got != "main" {
		t.Errorf("got %q, want %q", got, "main")
	}
}

func TestResolveBranchContextDetachedHEAD(t *testing.T) {
	dir := initRepo(t, "https://github.com/clay/my-project.git")

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("commit", "--allow-empty", "-m", "initial commit")
	run("checkout", "--detach", "HEAD")

	got, err := resolver.ResolveBranchContext(dir)
	if err != nil {
		t.Fatalf("ResolveBranchContext: %v", err)
	}
	if !strings.HasPrefix(got, "detached/") {
		t.Errorf("expected 'detached/' prefix for detached HEAD, got %q", got)
	}
}

func TestResolveBranchContextWorktree(t *testing.T) {
	dir := initRepo(t, "https://github.com/clay/my-project.git")

	run := func(repoDir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run(dir, "commit", "--allow-empty", "-m", "initial commit")
	run(dir, "branch", "feature/branch-ctx-test")

	wtDir := t.TempDir()
	run(dir, "worktree", "add", wtDir, "feature/branch-ctx-test")

	got, err := resolver.ResolveBranchContext(wtDir)
	if err != nil {
		t.Fatalf("ResolveBranchContext in worktree: %v", err)
	}
	want := "feature/branch-ctx-test"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveBranchContextNotInGitRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := resolver.ResolveBranchContext(dir)
	if err == nil {
		t.Fatal("expected error for non-git directory, got nil")
	}
	if !strings.Contains(err.Error(), "git repository") {
		t.Errorf("error should mention 'git repository', got: %v", err)
	}
}

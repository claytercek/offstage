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

// mustGit runs a git command in dir, failing the test on error.
func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	fullArgs := append([]string{"-C", dir}, args...)
	out, err := exec.Command("git", fullArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// setupBareRemote initialises a bare git repo to act as the remote.
func setupBareRemote(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustGit(t, dir, "init", "--bare")
	return dir
}

// setupLocalStore clones the bare remote into a local store directory.
func setupLocalStore(t *testing.T, remoteDir string) *store.Store {
	t.Helper()
	localDir := filepath.Join(t.TempDir(), "store")
	out, err := exec.Command("git", "clone", remoteDir, localDir).CombinedOutput()
	if err != nil {
		t.Fatalf("git clone: %v\n%s", err, out)
	}
	mustGit(t, localDir, "config", "user.email", "test@test.com")
	mustGit(t, localDir, "config", "user.name", "Test")

	// Create an initial commit so the repo has a HEAD to branch from.
	readmePath := filepath.Join(localDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("offstage store\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, localDir, "add", ".")
	mustGit(t, localDir, "commit", "-m", "init")
	mustGit(t, localDir, "push", "origin", "HEAD")

	s, err := store.Open(localDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return s
}

// setupProjectDir creates a fake project directory with tracked files.
func setupProjectDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, "CONTEXT.md", "# Context\nsome context here\n")
	writeFile(t, dir, ".agents/rules.md", "# Rules\nsome rules here\n")
	return dir
}

func writeFile(t *testing.T, base, relPath, content string) {
	t.Helper()
	full := filepath.Join(base, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPush_BasicFlow(t *testing.T) {
	remoteDir := setupBareRemote(t)
	s := setupLocalStore(t, remoteDir)
	projectDir := setupProjectDir(t)

	include := []string{"CONTEXT.md", ".agents/**"}
	exclude := []string{}
	projectID := "github.com/test/my-project"
	branchName := "main"

	err := syncer.Push(s, projectDir, include, exclude, projectID, branchName, false)
	if err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	// Verify the branch was created.
	storeBranch := projectID + "/" + branchName
	if !s.BranchExists(storeBranch) {
		t.Errorf("expected branch %q to exist after push", storeBranch)
	}

	// Checkout the branch and verify files exist.
	if err := s.Checkout(storeBranch); err != nil {
		t.Fatalf("checkout store branch: %v", err)
	}

	// Verify CONTEXT.md exists in the store with correct content.
	contextPath := filepath.Join(s.Path, "CONTEXT.md")
	data, err := os.ReadFile(contextPath)
	if err != nil {
		t.Fatalf("read CONTEXT.md from store: %v", err)
	}
	if !strings.Contains(string(data), "some context here") {
		t.Errorf("CONTEXT.md content mismatch: got %q", string(data))
	}

	// Verify .agents/rules.md exists in the store with correct content.
	rulesPath := filepath.Join(s.Path, ".agents/rules.md")
	data, err = os.ReadFile(rulesPath)
	if err != nil {
		t.Fatalf("read .agents/rules.md from store: %v", err)
	}
	if !strings.Contains(string(data), "some rules here") {
		t.Errorf(".agents/rules.md content mismatch: got %q", string(data))
	}
}

func TestPush_NothingToCommit(t *testing.T) {
	remoteDir := setupBareRemote(t)
	s := setupLocalStore(t, remoteDir)
	projectDir := setupProjectDir(t)

	include := []string{"CONTEXT.md", ".agents/**"}
	exclude := []string{}
	projectID := "github.com/test/my-project"
	branchName := "main"

	// First push.
	if err := syncer.Push(s, projectDir, include, exclude, projectID, branchName, false); err != nil {
		t.Fatalf("first Push failed: %v", err)
	}

	// Second push with no changes — should succeed cleanly.
	if err := syncer.Push(s, projectDir, include, exclude, projectID, branchName, false); err != nil {
		t.Fatalf("second Push (nothing to push) failed: %v", err)
	}
}

func TestPush_DryRun(t *testing.T) {
	remoteDir := setupBareRemote(t)
	s := setupLocalStore(t, remoteDir)
	projectDir := setupProjectDir(t)

	include := []string{"CONTEXT.md", ".agents/**"}
	exclude := []string{}
	projectID := "github.com/test/my-project"
	branchName := "main"

	err := syncer.Push(s, projectDir, include, exclude, projectID, branchName, true /* dryRun */)
	if err != nil {
		t.Fatalf("dry-run Push failed: %v", err)
	}

	// After a dry-run, CONTEXT.md should NOT exist in the store working tree.
	// (Regardless of branch state, no files should have been copied.)
	contextPath := filepath.Join(s.Path, "CONTEXT.md")
	if _, err := os.Stat(contextPath); err == nil {
		t.Error("dry-run should not copy files to store, but CONTEXT.md was found")
	}
}

func TestCollectFiles_BasicPatterns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "CONTEXT.md", "context")
	writeFile(t, dir, "AGENTS.md", "agents")
	writeFile(t, dir, ".agents/rules.md", "rules")
	writeFile(t, dir, ".agents/sub/deep.md", "deep")
	writeFile(t, dir, "docs/adr/0001-foo.md", "adr")
	writeFile(t, dir, "untracked.txt", "ignored")

	include := []string{"CONTEXT.md", "AGENTS.md", ".agents/**", "docs/adr/**"}
	exclude := []string{}

	files, err := syncer.CollectFiles(dir, include, exclude)
	if err != nil {
		t.Fatalf("CollectFiles: %v", err)
	}

	want := map[string]bool{
		"CONTEXT.md":           true,
		"AGENTS.md":            true,
		".agents/rules.md":     true,
		".agents/sub/deep.md":  true,
		"docs/adr/0001-foo.md": true,
	}
	for _, f := range files {
		if !want[f] {
			t.Errorf("unexpected file %q in result", f)
		}
		delete(want, f)
	}
	for f := range want {
		t.Errorf("expected file %q not found in result", f)
	}
}

func TestCollectFiles_Exclude(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "CONTEXT.md", "context")
	writeFile(t, dir, ".agents/rules.md", "rules")
	writeFile(t, dir, ".agents/secret.md", "secret")

	include := []string{"CONTEXT.md", ".agents/**"}
	exclude := []string{".agents/secret.md"}

	files, err := syncer.CollectFiles(dir, include, exclude)
	if err != nil {
		t.Fatalf("CollectFiles: %v", err)
	}

	for _, f := range files {
		if f == ".agents/secret.md" {
			t.Error("excluded file .agents/secret.md was included")
		}
	}
}

// TestPush_StaleFilesRemovedAfterUntrack verifies that when include patterns
// are narrowed (simulating "offstage untrack"), a subsequent push removes files
// from the store that no longer match the new patterns.
func TestPush_StaleFilesRemovedAfterUntrack(t *testing.T) {
	remoteDir := setupBareRemote(t)
	s := setupLocalStore(t, remoteDir)
	projectDir := setupProjectDir(t) // has CONTEXT.md and .agents/rules.md

	projectID := "github.com/test/my-project"
	branchName := "main"

	// First push: include both CONTEXT.md and .agents/**
	include1 := []string{"CONTEXT.md", ".agents/**"}
	if err := syncer.Push(s, projectDir, include1, nil, projectID, branchName, false); err != nil {
		t.Fatalf("first Push failed: %v", err)
	}

	storeBranch := projectID + "/" + branchName

	// Confirm .agents/rules.md was committed in the store.
	if err := s.Checkout(storeBranch); err != nil {
		t.Fatalf("checkout store branch: %v", err)
	}
	rulesInStore := filepath.Join(s.Path, ".agents/rules.md")
	if _, err := os.Stat(rulesInStore); err != nil {
		t.Fatalf(".agents/rules.md should exist in store after first push: %v", err)
	}

	// Second push: drop .agents/** (simulate untrack). Only CONTEXT.md is tracked now.
	include2 := []string{"CONTEXT.md"}
	if err := syncer.Push(s, projectDir, include2, nil, projectID, branchName, false); err != nil {
		t.Fatalf("second Push (after untrack) failed: %v", err)
	}

	// After the second push, .agents/rules.md must NOT exist in the store.
	if err := s.Checkout(storeBranch); err != nil {
		t.Fatalf("checkout store branch after second push: %v", err)
	}
	if _, err := os.Stat(rulesInStore); err == nil {
		t.Error(".agents/rules.md still exists in store after untrack + push; stale file was not removed")
	}

	// CONTEXT.md must still exist in the store.
	contextInStore := filepath.Join(s.Path, "CONTEXT.md")
	if _, err := os.Stat(contextInStore); err != nil {
		t.Errorf("CONTEXT.md should still exist in store after second push: %v", err)
	}
}

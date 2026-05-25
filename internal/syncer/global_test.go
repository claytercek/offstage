package syncer_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gastownhall/offstage/internal/store"
	"github.com/gastownhall/offstage/internal/syncer"
)

// setupHomeDir creates a fake home directory with the given files.
func setupHomeDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		writeFile(t, dir, rel, content)
	}
	return dir
}

// seedGlobalBranch is like seedStoreBranch but uses --force when staging
// so that files in directories like .claude/ are not blocked by gitignore.
func seedGlobalBranch(t *testing.T, s *store.Store, storeBranch string, files map[string]string) {
	t.Helper()

	defaultBranch := currentBranch(t, s.Path)

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

	// Use --force to bypass gitignore rules (e.g. for .claude/ files).
	mustRun(t, "git", "-C", s.Path, "add", "--force", "-A")
	mustRun(t, "git", "-C", s.Path, "commit", "-m", "seed global files for pull test")

	if err := s.Exec("push", "origin", storeBranch); err != nil {
		t.Fatalf("push store branch %s: %v", storeBranch, err)
	}

	if err := s.Exec("checkout", defaultBranch); err != nil {
		t.Fatalf("checkout default branch: %v", err)
	}
}

// TestPushGlobal_BasicFlow verifies that PushGlobal copies files to the
// "global" branch in the store.
func TestPushGlobal_BasicFlow(t *testing.T) {
	remoteDir := setupBareRemote(t)
	s := setupLocalStore(t, remoteDir)

	homeDir := setupHomeDir(t, map[string]string{
		".claude/CLAUDE.md": "# Global CLAUDE.md\ncontent here\n",
	})

	include := []string{".claude/CLAUDE.md"}

	if err := syncer.PushGlobal(s, homeDir, include, false); err != nil {
		t.Fatalf("PushGlobal: %v", err)
	}

	// Verify the global branch was created.
	if !s.BranchExists(syncer.GlobalBranch) {
		t.Errorf("expected branch %q to exist after PushGlobal", syncer.GlobalBranch)
	}

	// Checkout the branch and verify the file exists.
	if err := s.Checkout(syncer.GlobalBranch); err != nil {
		t.Fatalf("checkout global branch: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(s.Path, ".claude", "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read .claude/CLAUDE.md from store: %v", err)
	}
	if string(data) != "# Global CLAUDE.md\ncontent here\n" {
		t.Errorf(".claude/CLAUDE.md content mismatch: got %q", string(data))
	}
}

// TestPullGlobal_BasicFlow seeds the global branch and verifies PullGlobal
// restores files to the home directory.
func TestPullGlobal_BasicFlow(t *testing.T) {
	s, _ := initBareRemoteAndStore(t)

	// Seed the global branch using --force to bypass gitignore for .claude/.
	seedGlobalBranch(t, s, syncer.GlobalBranch, map[string]string{
		".claude/CLAUDE.md": "# Pulled CLAUDE.md\n",
	})

	// Pull to a fresh home directory.
	homeDir := t.TempDir()
	if err := syncer.PullGlobal(s, homeDir, false); err != nil {
		t.Fatalf("PullGlobal: %v", err)
	}

	// Verify the file was restored.
	checkFile(t, filepath.Join(homeDir, ".claude", "CLAUDE.md"), "# Pulled CLAUDE.md\n")
}

// TestPushGlobal_DryRun verifies that PushGlobal with dryRun=true does not
// copy files to the store.
func TestPushGlobal_DryRun(t *testing.T) {
	remoteDir := setupBareRemote(t)
	s := setupLocalStore(t, remoteDir)

	homeDir := setupHomeDir(t, map[string]string{
		".claude/CLAUDE.md": "# DryRun content\n",
	})

	include := []string{".claude/CLAUDE.md"}

	if err := syncer.PushGlobal(s, homeDir, include, true /* dryRun */); err != nil {
		t.Fatalf("PushGlobal dry-run: %v", err)
	}

	// After a dry-run, .claude/CLAUDE.md should NOT exist in the store working tree.
	claudePath := filepath.Join(s.Path, ".claude", "CLAUDE.md")
	if _, err := os.Stat(claudePath); err == nil {
		t.Error("dry-run should not copy files to store, but .claude/CLAUDE.md was found")
	}
}

// TestPullGlobal_BranchNotFound verifies that PullGlobal returns ErrBranchNotFound
// when the global branch has never been pushed.
func TestPullGlobal_BranchNotFound(t *testing.T) {
	s, _ := initBareRemoteAndStore(t)

	homeDir := t.TempDir()
	err := syncer.PullGlobal(s, homeDir, false)
	if err == nil {
		t.Fatal("expected ErrBranchNotFound, got nil")
	}
	if !errors.Is(err, syncer.ErrBranchNotFound) {
		t.Errorf("expected ErrBranchNotFound, got: %v", err)
	}
}

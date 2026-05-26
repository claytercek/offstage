package syncer

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/claytercek/offstage/internal/store"
)

// ErrHasDiff signals that differences were found (exit code 1 per POSIX diff convention).
var ErrHasDiff = errors.New("differences found")

// DiffLocal shows the unified diff between local tracked files and the sync store source state.
// Returns ErrHasDiff if there are differences (to produce exit code 1).
func DiffLocal(s *store.Store, projectDir string, include []string, exclude []string, storeBranch string) error {
	// Check branch exists.
	if !s.BranchExists(storeBranch) {
		return fmt.Errorf("no sync data for branch %s; run 'offstage push' first", storeBranch)
	}

	// Checkout the store branch so files are on disk.
	if err := s.Checkout(storeBranch); err != nil {
		return fmt.Errorf("checkout store branch %s: %w", storeBranch, err)
	}

	// Collect tracked files from the project directory.
	files, err := CollectFiles(projectDir, include, exclude)
	if err != nil {
		return fmt.Errorf("collect files: %w", err)
	}

	hasDiff := false

	for _, relPath := range files {
		storePath := filepath.Join(s.Path, relPath)
		localPath := filepath.Join(projectDir, relPath)

		// If the store doesn't have this file at all, it's a difference.
		if _, statErr := os.Stat(storePath); os.IsNotExist(statErr) {
			fmt.Printf("--- /dev/null\n+++ b/%s\n", relPath)
			data, readErr := os.ReadFile(localPath)
			if readErr == nil {
				lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
				fmt.Printf("@@ -0,0 +1,%d @@\n", len(lines))
				for _, line := range lines {
					fmt.Printf("+%s\n", line)
				}
			}
			hasDiff = true
			continue
		}

		// Run git diff --no-index <store-file> <local-file> and forward output.
		// git diff --no-index exits 1 when files differ, 0 when identical.
		cmd := exec.Command("git", "diff", "--no-index", storePath, localPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if runErr := cmd.Run(); runErr != nil {
			// Exit code 1 means differences found; any other error is a real failure.
			var exitErr *exec.ExitError
			if errors.As(runErr, &exitErr) && exitErr.ExitCode() == 1 {
				hasDiff = true
			} else {
				return fmt.Errorf("git diff --no-index %s %s: %w", storePath, localPath, runErr)
			}
		}
	}

	if hasDiff {
		return ErrHasDiff
	}
	return nil
}

// DiffBranches shows the unified diff between two sync store branches.
// Returns ErrHasDiff if there are differences (to produce exit code 1).
func DiffBranches(s *store.Store, currentBranch, targetBranch string) error {
	// Check both branches exist.
	if !s.BranchExists(currentBranch) {
		return fmt.Errorf("branch %q not found in sync store; run 'offstage push' first", currentBranch)
	}
	if !s.BranchExists(targetBranch) {
		return fmt.Errorf("branch %q not found in sync store", targetBranch)
	}

	// Check if there are differences using --name-only (quiet check).
	nameOnlyOut, err := exec.Command("git", "-C", s.Path, "diff", "--name-only", currentBranch, targetBranch).Output()
	if err != nil {
		return fmt.Errorf("git diff --name-only %s %s: %w", currentBranch, targetBranch, err)
	}

	hasDiff := strings.TrimSpace(string(nameOnlyOut)) != ""

	if hasDiff {
		// Forward the unified diff to stdout.
		cmd := exec.Command("git", "-C", s.Path, "diff", currentBranch, targetBranch)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if runErr := cmd.Run(); runErr != nil {
			return fmt.Errorf("git diff %s %s: %w", currentBranch, targetBranch, runErr)
		}
		return ErrHasDiff
	}

	return nil
}

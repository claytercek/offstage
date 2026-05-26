package syncer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/claytercek/offstage/internal/store"
)

// Push copies the tracked files from projectDir into the sync store branch
// for this project+branch and pushes to remote.
// Branch name format: <project-id>/<branch-name>
// (e.g. "github.com/clay/my-project/main")
func Push(s *store.Store, projectDir string, include []string, exclude []string, projectID string, branchName string, dryRun bool) error {
	storeBranch := projectID + "/" + branchName

	// Fetch from origin so we know about remote branches.
	if err := s.Fetch(); err != nil {
		// Non-fatal: may be offline or no remote yet; continue.
		fmt.Fprintf(os.Stderr, "warning: fetch failed: %v\n", err)
	}

	// Create or checkout the store branch.
	if s.BranchExists(storeBranch) {
		if err := s.Checkout(storeBranch); err != nil {
			return fmt.Errorf("checkout store branch %s: %w", storeBranch, err)
		}
	} else {
		if err := s.CreateBranch(storeBranch); err != nil {
			return fmt.Errorf("create store branch %s: %w", storeBranch, err)
		}
	}

	// Collect files matching include/exclude patterns.
	files, err := CollectFiles(projectDir, include, exclude)
	if err != nil {
		return fmt.Errorf("collect files: %w", err)
	}

	if dryRun {
		if len(files) == 0 {
			fmt.Println("nothing to push (no files matched)")
			return nil
		}
		fmt.Println("files that would be pushed:")
		for _, f := range files {
			fmt.Printf("  %s\n", f)
		}
		return nil
	}

	// Build a set of currently tracked relative paths for fast lookup.
	tracked := make(map[string]bool, len(files))
	for _, f := range files {
		tracked[f] = true
	}

	// Remove any files present in the store working tree that are no longer
	// in the tracked set. This ensures that untrack takes effect immediately
	// and the store does not accumulate stale files across pushes.
	if err := removeStaleStoreFiles(s.Path, tracked); err != nil {
		return fmt.Errorf("remove stale store files: %w", err)
	}

	// Copy each file to the same relative path in the store working tree.
	for _, relPath := range files {
		src := filepath.Join(projectDir, relPath)
		dst := filepath.Join(s.Path, relPath)

		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("create parent dirs for %s: %w", relPath, err)
		}

		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("copy %s: %w", relPath, err)
		}
	}

	// Stage all changes. Use --force to override any global gitignore rules
	// that might exclude tracked files (e.g. .agents/ is commonly gitignored).
	if err := s.Exec("add", "--force", "."); err != nil {
		return fmt.Errorf("stage changes: %w", err)
	}

	// Check if there are any staged changes.
	staged, err := s.ExecOutput("diff", "--staged", "--name-only")
	if err != nil {
		return fmt.Errorf("check staged changes: %w", err)
	}
	if strings.TrimSpace(staged) == "" {
		fmt.Println("nothing to push")
		return nil
	}

	// Commit.
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	message := fmt.Sprintf("sync: %s/%s from %s", projectID, branchName, hostname)
	if err := s.Commit(message); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	// Push to remote.
	if err := s.Push(); err != nil {
		return fmt.Errorf("push to remote: %w", err)
	}

	return nil
}

// removeStaleStoreFiles walks the store working tree and deletes any file whose
// relative path is not present in the tracked set. The .git directory is
// always skipped.
func removeStaleStoreFiles(storePath string, tracked map[string]bool) error {
	return filepath.WalkDir(storePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, relErr := filepath.Rel(storePath, path)
		if relErr != nil {
			return relErr
		}

		// Always skip the .git directory.
		if relPath == ".git" || strings.HasPrefix(relPath, ".git"+string(filepath.Separator)) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}

		// If this file is not in the tracked set, remove it.
		if !tracked[relPath] {
			if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
				return fmt.Errorf("remove stale file %s: %w", relPath, removeErr)
			}
		}
		return nil
	})
}

// CollectFiles walks projectDir and returns relative paths of files matching
// any include pattern and not matching any exclude pattern.
func CollectFiles(projectDir string, include []string, exclude []string) ([]string, error) {
	var result []string

	err := filepath.WalkDir(projectDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(projectDir, path)
		if err != nil {
			return err
		}

		if matchesAny(relPath, include) && !matchesAny(relPath, exclude) {
			result = append(result, relPath)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// matchesAny returns true if path matches any of the glob patterns.
// Supports ** for recursive matching.
func matchesAny(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchGlob(pattern, path) {
			return true
		}
	}
	return false
}

// matchGlob matches a single pattern against path, supporting ** for
// recursive directory matching.
func matchGlob(pattern, path string) bool {
	if !strings.Contains(pattern, "**") {
		ok, _ := filepath.Match(pattern, path)
		return ok
	}

	// Split on "/**" to get prefix and suffix.
	parts := strings.SplitN(pattern, "/**", 2)
	prefix := parts[0]

	if prefix == "" {
		// Pattern is "**" or "/**..." — matches everything.
		return true
	}

	// path must start with prefix+"/" or equal prefix.
	return strings.HasPrefix(path, prefix+"/") || path == prefix
}


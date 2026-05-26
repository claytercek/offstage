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

// FileStatus holds the sync status of a single tracked file.
type FileStatus struct {
	Path  string
	State string // "modified", "new file", "deleted"
}

// Status compares the local tracked files against the sync store source state
// and prints a per-file summary. Returns ErrHasDiff if any differences exist.
func Status(s *store.Store, projectDir string, include []string, exclude []string, storeBranch string) error {
	var fs FileSet
	files, err := fs.Collect(projectDir, include, exclude)
	if err != nil {
		return fmt.Errorf("collect files: %w", err)
	}

	tracked := make(map[string]bool, len(files))
	for _, f := range files {
		tracked[f] = true
	}

	// When the store branch doesn't exist yet, everything local is new.
	if !s.BranchExists(storeBranch) {
		var statuses []FileStatus
		for _, f := range files {
			statuses = append(statuses, FileStatus{Path: f, State: "new file"})
		}
		printStatuses(statuses)
		if len(statuses) > 0 {
			return ErrHasDiff
		}
		fmt.Println("everything in sync")
		return nil
	}

	if err := s.Checkout(storeBranch); err != nil {
		return fmt.Errorf("checkout store branch %s: %w", storeBranch, err)
	}

	var statuses []FileStatus

	for _, relPath := range files {
		storePath := filepath.Join(s.Path, relPath)
		localPath := filepath.Join(projectDir, relPath)

		if _, statErr := os.Stat(storePath); os.IsNotExist(statErr) {
			statuses = append(statuses, FileStatus{Path: relPath, State: "new file"})
			continue
		}

		cmd := exec.Command("git", "diff", "--quiet", "--no-index", storePath, localPath)
		if runErr := cmd.Run(); runErr != nil {
			var exitErr *exec.ExitError
			if errors.As(runErr, &exitErr) && exitErr.ExitCode() == 1 {
				statuses = append(statuses, FileStatus{Path: relPath, State: "modified"})
			} else {
				return fmt.Errorf("compare %s: %w", relPath, runErr)
			}
		}
	}

	// Files in the store that are not in the tracked set would be deleted by push.
	walkErr := filepath.WalkDir(s.Path, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(s.Path, path)
		if relErr != nil {
			return relErr
		}
		if rel == ".git" || strings.HasPrefix(rel, ".git"+string(filepath.Separator)) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !tracked[rel] {
			statuses = append(statuses, FileStatus{Path: rel, State: "deleted"})
		}
		return nil
	})
	if walkErr != nil {
		return fmt.Errorf("walk store: %w", walkErr)
	}

	if len(statuses) == 0 {
		fmt.Println("everything in sync")
		return nil
	}

	printStatuses(statuses)
	return ErrHasDiff
}

func printStatuses(statuses []FileStatus) {
	for _, s := range statuses {
		fmt.Printf("  %-12s%s\n", s.State+":", s.Path)
	}
}

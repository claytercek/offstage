package syncer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/claytercek/offstage/internal/store"
)

// ErrBranchNotFound is returned when the store branch for this project doesn't exist yet.
var ErrBranchNotFound = errors.New("no sync data for this project yet; run 'offstage push' first")

// ErrDiverged is returned when the local and remote store branches have diverged.
var ErrDiverged = errors.New("sync store branch has diverged; use 'offstage git' commands to resolve manually")

// Pull fetches the sync store, finds the correct branch for this project+branch,
// and copies tracked files to projectDir.
// Returns ErrBranchNotFound if the branch doesn't exist yet in the store.
// Returns ErrDiverged if the branch has diverged (both local and remote have unique commits).
func Pull(s *store.Store, projectDir string, projectID string, branchName string, dryRun bool) error {
	storeBranch := projectID + "/" + branchName

	// Step 1: Fetch from origin.
	if err := s.Fetch(); err != nil {
		return fmt.Errorf("fetch from origin: %w", err)
	}

	localExists := s.BranchExists(storeBranch)

	// Check if remote tracking branch exists.
	remoteRef := "refs/remotes/origin/" + storeBranch
	_, remoteErr := s.ExecOutput("rev-parse", "--verify", remoteRef)
	remoteExists := remoteErr == nil

	if !localExists && !remoteExists {
		return ErrBranchNotFound
	}

	if !localExists && remoteExists {
		// Create local branch tracking the remote.
		if err := s.Exec("checkout", "-b", storeBranch, "origin/"+storeBranch); err != nil {
			return fmt.Errorf("checkout remote branch: %w", err)
		}
	} else if localExists {
		if remoteExists {
			// Check for diverged state by comparing local branch ref vs remote tracking ref.
			localRef := "refs/heads/" + storeBranch
			remoteAhead, err := s.ExecOutput("log", localRef+".."+remoteRef, "--oneline")
			if err != nil {
				return fmt.Errorf("check remote commits: %w", err)
			}
			localAhead, err := s.ExecOutput("log", remoteRef+".."+localRef, "--oneline")
			if err != nil {
				return fmt.Errorf("check local commits: %w", err)
			}

			remoteHasNew := strings.TrimSpace(remoteAhead) != ""
			localHasNew := strings.TrimSpace(localAhead) != ""

			if remoteHasNew && localHasNew {
				return ErrDiverged
			}

			if remoteHasNew {
				// Fast-forward: checkout then pull.
				if err := s.Checkout(storeBranch); err != nil {
					return fmt.Errorf("checkout store branch: %w", err)
				}
				if err := s.Pull(); err != nil {
					return fmt.Errorf("pull store branch: %w", err)
				}
			} else {
				// Local is ahead of or equal to remote — just checkout.
				if err := s.Checkout(storeBranch); err != nil {
					return fmt.Errorf("checkout store branch: %w", err)
				}
			}
		} else {
			// No remote branch — just checkout local.
			if err := s.Checkout(storeBranch); err != nil {
				return fmt.Errorf("checkout store branch: %w", err)
			}
		}
	}

	// Now the store is on the correct branch. List or copy files.
	if dryRun {
		return listStoreFiles(s.Path)
	}

	count, err := copyStoreFiles(s.Path, projectDir)
	if err != nil {
		return fmt.Errorf("copy files: %w", err)
	}
	fmt.Printf("pulled %d files from %s\n", count, storeBranch)
	return nil
}

// listStoreFiles prints all files in the store working tree (excluding .git).
func listStoreFiles(storeDir string) error {
	return filepath.Walk(storeDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(storeDir, path)
		if err != nil {
			return err
		}
		// Skip .git directory.
		if rel == ".git" || strings.HasPrefix(rel, ".git"+string(filepath.Separator)) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !info.IsDir() {
			fmt.Println(rel)
		}
		return nil
	})
}

// copyStoreFiles walks storeDir, skipping .git, and copies each file to the
// corresponding relative path under projectDir. Returns the number of files copied.
func copyStoreFiles(storeDir, projectDir string) (int, error) {
	count := 0
	err := filepath.Walk(storeDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(storeDir, path)
		if err != nil {
			return err
		}
		// Skip .git directory.
		if rel == ".git" || strings.HasPrefix(rel, ".git"+string(filepath.Separator)) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			return nil
		}

		dest := filepath.Join(projectDir, rel)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("create parent dir for %s: %w", dest, err)
		}
		if err := copyFile(path, dest); err != nil {
			return fmt.Errorf("copy %s: %w", rel, err)
		}
		count++
		return nil
	})
	return count, err
}


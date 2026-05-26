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

	// Step 2: Determine branch sync state and act accordingly.
	state, err := s.BranchSyncState(storeBranch)
	if err != nil {
		return fmt.Errorf("check branch state: %w", err)
	}

	switch state {
	case store.BranchNotFound:
		return ErrBranchNotFound

	case store.BranchDiverged:
		return ErrDiverged

	case store.BranchRemoteOnly:
		// Create a local branch tracking the remote.
		if err := s.CheckoutTrackRemote(storeBranch); err != nil {
			return fmt.Errorf("checkout remote branch: %w", err)
		}

	case store.BranchRemoteAhead:
		// Remote has new commits — fast-forward checkout + pull.
		if err := s.Checkout(storeBranch); err != nil {
			return fmt.Errorf("checkout store branch: %w", err)
		}
		if err := s.Pull(); err != nil {
			return fmt.Errorf("pull store branch: %w", err)
		}

	case store.BranchSynced, store.BranchLocalOnly:
		// Local is at or ahead of remote, or there is no remote — just checkout.
		if err := s.Checkout(storeBranch); err != nil {
			return fmt.Errorf("checkout store branch: %w", err)
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
// corresponding relative path under projectDir.
// Returns the number of files copied.
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
		if err := Copy(path, dest); err != nil {
			return fmt.Errorf("copy %s: %w", rel, err)
		}
		count++
		return nil
	})
	return count, err
}

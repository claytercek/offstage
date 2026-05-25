package syncer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gastownhall/offstage/internal/store"
)

// GlobalBranch is the sync store branch name used for globally-tracked files.
const GlobalBranch = "global"

// PushGlobal copies globally-tracked files from homeDir to the "global" branch
// in the sync store and pushes to remote.
func PushGlobal(s *store.Store, homeDir string, include []string, dryRun bool) error {
	// Create or checkout the global branch.
	if s.BranchExists(GlobalBranch) {
		if err := s.Checkout(GlobalBranch); err != nil {
			return fmt.Errorf("checkout global branch: %w", err)
		}
	} else {
		if err := s.CreateBranch(GlobalBranch); err != nil {
			return fmt.Errorf("create global branch: %w", err)
		}
	}

	// Collect files matching include patterns relative to homeDir.
	files, err := CollectFiles(homeDir, include, nil)
	if err != nil {
		return fmt.Errorf("collect global files: %w", err)
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

	// Copy each file to the same relative path in the store working tree.
	for _, relPath := range files {
		src := filepath.Join(homeDir, relPath)
		dst := filepath.Join(s.Path, relPath)

		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("create parent dirs for %s: %w", relPath, err)
		}

		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("copy %s: %w", relPath, err)
		}
	}

	// Stage all changes with --force to override gitignore rules.
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
	message := fmt.Sprintf("sync: global from %s", hostname)
	if err := s.Commit(message); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	// Push to remote.
	if err := s.Push(); err != nil {
		return fmt.Errorf("push to remote: %w", err)
	}

	return nil
}

// PullGlobal copies files from the "global" sync store branch back to homeDir.
// Returns ErrBranchNotFound if the global branch doesn't exist yet.
func PullGlobal(s *store.Store, homeDir string, dryRun bool) error {
	// Step 1: Fetch from origin.
	if err := s.Fetch(); err != nil {
		return fmt.Errorf("fetch from origin: %w", err)
	}

	localExists := s.BranchExists(GlobalBranch)

	// Check if remote tracking branch exists.
	remoteRef := "refs/remotes/origin/" + GlobalBranch
	_, remoteErr := s.ExecOutput("rev-parse", "--verify", remoteRef)
	remoteExists := remoteErr == nil

	if !localExists && !remoteExists {
		return ErrBranchNotFound
	}

	if !localExists && remoteExists {
		// Create local branch tracking the remote.
		if err := s.Exec("checkout", "-b", GlobalBranch, "origin/"+GlobalBranch); err != nil {
			return fmt.Errorf("checkout remote global branch: %w", err)
		}
	} else if localExists {
		if remoteExists {
			// Check for diverged state.
			localRef := "refs/heads/" + GlobalBranch
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
				if err := s.Checkout(GlobalBranch); err != nil {
					return fmt.Errorf("checkout global branch: %w", err)
				}
				if err := s.Pull(); err != nil {
					return fmt.Errorf("pull global branch: %w", err)
				}
			} else {
				// Local is ahead of or equal to remote — just checkout.
				if err := s.Checkout(GlobalBranch); err != nil {
					return fmt.Errorf("checkout global branch: %w", err)
				}
			}
		} else {
			// No remote branch — just checkout local.
			if err := s.Checkout(GlobalBranch); err != nil {
				return fmt.Errorf("checkout global branch: %w", err)
			}
		}
	}

	// Now the store is on the global branch. List or copy files.
	if dryRun {
		return listStoreFiles(s.Path)
	}

	count, err := copyStoreFiles(s.Path, homeDir)
	if err != nil {
		return fmt.Errorf("copy files: %w", err)
	}
	fmt.Printf("pulled %d files from %s\n", count, GlobalBranch)
	return nil
}

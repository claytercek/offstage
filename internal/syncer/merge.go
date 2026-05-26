package syncer

import (
	"errors"
	"fmt"
	"strings"

	"github.com/claytercek/offstage/internal/registry"
	"github.com/claytercek/offstage/internal/store"
)

// ErrMergeConflict is returned when the merge produces conflicts.
// The conflicting files are listed in the error message.
var ErrMergeConflict = errors.New("merge conflict")

// Merge merges the sync store branch for sourceBranch into the current branch's
// sync store branch for the given project.
//
// Branch names in store: <projectID>/<branchName>
//
// On clean merge: pushes to remote, marks sourceBranch as reconciled in registry.
// On conflict: returns ErrMergeConflict with conflicting files listed; does NOT push.
func Merge(s *store.Store, projectID, currentBranch, sourceBranch string) error {
	targetStoreBranch := projectID + "/" + currentBranch
	sourceStoreBranch := projectID + "/" + sourceBranch

	// Step 1: Fetch from origin to ensure we have the latest state.
	if err := s.Fetch(); err != nil {
		// Non-fatal: may be offline; continue with local state.
		fmt.Printf("warning: fetch failed: %v\n", err)
	}

	// Step 2: Check target branch exists.
	if !s.BranchExists(targetStoreBranch) {
		return fmt.Errorf("no sync data for current branch; run 'offstage push' first")
	}

	// Step 3: Check source branch exists.
	if !s.BranchExists(sourceStoreBranch) {
		return fmt.Errorf("no sync data for branch %s; run 'offstage push' on that branch first", sourceBranch)
	}

	// Step 4: Checkout the target branch.
	if err := s.Checkout(targetStoreBranch); err != nil {
		return fmt.Errorf("checkout target branch %s: %w", targetStoreBranch, err)
	}

	// Step 5: Merge source branch into target.
	mergeErr := s.MergeBranch(sourceStoreBranch)
	if mergeErr != nil {
		// Get conflicting files.
		conflictOut, _ := s.ExecOutput("diff", "--name-only", "--diff-filter=U")
		conflictFiles := strings.TrimSpace(conflictOut)

		// Abort the merge to clean up.
		_ = s.Exec("merge", "--abort")

		var fileList string
		if conflictFiles != "" {
			fileList = "\n  " + strings.ReplaceAll(conflictFiles, "\n", "\n  ")
		}
		return fmt.Errorf("%w: conflicting files:%s\n\nResolve manually using 'offstage git' commands:\n  offstage git checkout %s\n  offstage git merge %s\n  # resolve conflicts, then:\n  offstage git add <files>\n  offstage git commit\n  offstage git push origin HEAD",
			ErrMergeConflict, fileList, targetStoreBranch, sourceStoreBranch)
	}

	// Step 6: Push to remote.
	if err := s.Push(); err != nil {
		return fmt.Errorf("push after merge: %w", err)
	}

	// Step 7: Record reconciliation in manifest on store main branch.
	if err := recordReconciliation(s, projectID, currentBranch, sourceBranch, targetStoreBranch); err != nil {
		// Non-fatal: warn but don't fail the merge.
		fmt.Printf("warning: could not update reconciliation registry: %v\n", err)
	}

	return nil
}

// recordReconciliation switches to the main branch in the store, loads the
// manifest, marks sourceBranch as reconciled, commits, pushes, and restores
// the working branch.
func recordReconciliation(s *store.Store, projectID, currentBranch, sourceBranch, restoreBranch string) error {
	// Switch to (or create) the main branch.
	if s.BranchExists("main") {
		if err := s.Checkout("main"); err != nil {
			return fmt.Errorf("checkout main: %w", err)
		}
	} else {
		if err := s.CreateBranch("main"); err != nil {
			return fmt.Errorf("create main branch: %w", err)
		}
	}

	// Restore the original branch on exit regardless of outcome.
	defer func() {
		_ = s.Checkout(restoreBranch)
	}()

	// Load or create manifest.
	m, err := registry.Load(s.Path)
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}

	// Ensure both branches are registered, then mark source as reconciled.
	registry.Register(m, projectID, currentBranch)
	modified := registry.MarkReconciled(m, projectID, sourceBranch)
	if !modified {
		// Already reconciled — nothing to commit.
		return nil
	}

	// Save, stage, commit, push.
	if err := registry.Save(s.Path, m); err != nil {
		return fmt.Errorf("save manifest: %w", err)
	}
	if err := s.Exec("add", "--force", "manifest.toml"); err != nil {
		return fmt.Errorf("stage manifest: %w", err)
	}
	msg := fmt.Sprintf("registry: reconcile %s/%s into %s/%s", projectID, sourceBranch, projectID, currentBranch)
	if err := s.Commit(msg); err != nil {
		return fmt.Errorf("commit manifest: %w", err)
	}
	if err := s.Push(); err != nil {
		return fmt.Errorf("push manifest: %w", err)
	}

	return nil
}

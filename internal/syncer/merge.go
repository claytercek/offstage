package syncer

import (
	"fmt"
	"strings"

	"github.com/claytercek/offstage/internal/store"
)

// Merge merges the sync store branch for sourceBranch into the current branch's
// sync store branch for the given project.
//
// Branch names in store: <projectID>/<branchName>
//
// On clean merge: pushes to remote.
// On conflict: leaves the merge in-progress with conflict markers and returns a
// descriptive error with offstage git commands for resolution. Does NOT push.
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
		conflictOut, _ := s.ConflictedFiles()
		conflictFiles := strings.TrimSpace(conflictOut)

		// Leave the merge in-progress so the user can resolve conflicts directly.
		// Do NOT abort — the user needs the conflict markers in place.

		var fileList string
		if conflictFiles != "" {
			fileList = "\n  " + strings.ReplaceAll(conflictFiles, "\n", "\n  ")
		}
		return fmt.Errorf("merge conflict: conflicting files:%s\n\nResolve conflicts then complete the merge:\n  offstage git add <files>\n  offstage git commit\n  offstage git push origin HEAD\n\nOr abort the merge with:\n  offstage git merge --abort",
			fileList)
	}

	// Step 6: Push to remote.
	if err := s.Push(); err != nil {
		return fmt.Errorf("push after merge: %w", err)
	}

	return nil
}

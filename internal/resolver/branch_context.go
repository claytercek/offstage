package resolver

import (
	"fmt"
)

// ResolveBranchContext returns the current branch name for the git repo
// containing dir. In detached HEAD state the result is prefixed with
// "detached/" followed by the short commit hash. Supports linked worktrees.
func ResolveBranchContext(dir string) (string, error) {
	if _, err := gitOutput(dir, "rev-parse", "--show-toplevel"); err != nil {
		return "", fmt.Errorf("not inside a git repository: %w", err)
	}

	branch, err := gitOutput(dir, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		// Detached HEAD — fall back to short commit hash.
		hash, hashErr := gitOutput(dir, "rev-parse", "--short", "HEAD")
		if hashErr != nil {
			return "", fmt.Errorf("resolve current branch: %w", hashErr)
		}
		return "detached/" + hash, nil
	}

	return branch, nil
}

package resolver

import (
	"fmt"
	"path/filepath"
)

// RepositoryRoot returns the top-level directory of the git repository
// containing dir.
func RepositoryRoot(dir string) (string, error) {
	root, err := gitOutput(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not inside a git repository: %w", err)
	}
	return root, nil
}

// GitPath resolves a path inside git's private directory for the repository
// containing dir. It supports linked worktrees where .git is a file.
func GitPath(dir, path string) (string, error) {
	resolved, err := gitOutput(dir, "rev-parse", "--git-path", path)
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(resolved) {
		return resolved, nil
	}

	root, err := RepositoryRoot(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, resolved), nil
}

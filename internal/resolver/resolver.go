// Package resolver identifies the current project and branch context so that
// offstage knows which branch of the sync store to read/write.
package resolver

import (
	"os/exec"
	"strings"
)

// Result holds the resolved project identity and branch name.
type Result struct {
	// RepoRoot is the top-level directory of the git repository.
	RepoRoot string
	// ProjectID is a normalized identifier derived from the git remote "origin",
	// e.g. "github.com/clay/my-project". Can be overridden by .offstagerc.toml.
	ProjectID string
	// BranchName is the current branch, e.g. "main" or "feature/foo".
	// In detached HEAD state it is prefixed with "detached/".
	BranchName string
}

// Resolve returns the project identity and current branch for the git repo
// containing dir. Returns an error if dir is not inside a git repo.
func Resolve(dir string) (*Result, error) {
	repoRoot, err := RepositoryRoot(dir)
	if err != nil {
		return nil, err
	}

	projectID, err := ResolveProjectID(dir)
	if err != nil {
		return nil, err
	}

	branch, err := ResolveBranchContext(dir)
	if err != nil {
		return nil, err
	}

	return &Result{
		RepoRoot:   repoRoot,
		ProjectID:  projectID,
		BranchName: branch,
	}, nil
}

// gitOutput runs a git command in dir and returns trimmed stdout, or an error.
func gitOutput(dir string, args ...string) (string, error) {
	gitBin, err := exec.LookPath("git")
	if err != nil {
		gitBin = "/usr/bin/git"
	}
	cmd := exec.Command(gitBin, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Package resolver identifies the current project and branch context so that
// offstage knows which branch of the sync store to read/write.
package resolver

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Result holds the resolved project identity and branch name.
type Result struct {
	// ProjectID is a normalized identifier derived from the git remote "origin",
	// e.g. "github.com/clay/my-project". Can be overridden by .offstagerc.toml.
	ProjectID string
	// BranchName is the current branch, e.g. "main" or "feature/foo".
	// In detached HEAD state it is prefixed with "detached/".
	BranchName string
}

// projectConfigName is a minimal struct for reading the name field from
// .offstagerc.toml without importing the full manifest package.
type projectConfigName struct {
	Name string `toml:"name"`
}

// Resolve returns the project identity and current branch for the git repo
// containing dir. Returns an error if dir is not inside a git repo.
func Resolve(dir string) (*Result, error) {
	// Find the repo root (also works inside worktrees).
	repoRoot, err := gitOutput(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("not inside a git repository: %w", err)
	}

	// Determine branch name.
	branch, err := gitOutput(dir, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		// Detached HEAD — fall back to short commit hash.
		hash, hashErr := gitOutput(dir, "rev-parse", "--short", "HEAD")
		if hashErr != nil {
			return nil, fmt.Errorf("resolve current branch: %w", hashErr)
		}
		branch = "detached/" + hash
	}

	// Check for name override in .offstagerc.toml at the repo root.
	rcPath := filepath.Join(repoRoot, ".offstagerc.toml")
	if _, statErr := os.Stat(rcPath); statErr == nil {
		var cfg projectConfigName
		if _, decodeErr := toml.DecodeFile(rcPath, &cfg); decodeErr == nil && cfg.Name != "" {
			return &Result{
				ProjectID:  cfg.Name,
				BranchName: branch,
			}, nil
		}
	}

	// Derive project ID from the "origin" remote URL.
	remoteURL, err := gitOutput(dir, "remote", "get-url", "origin")
	if err != nil {
		return nil, fmt.Errorf("no remote named \"origin\"; run 'offstage init <git-url>' to configure one")
	}

	projectID := normalizeRemoteURL(remoteURL)

	return &Result{
		ProjectID:  projectID,
		BranchName: branch,
	}, nil
}

// normalizeRemoteURL converts a git remote URL into a canonical project ID.
//
// Rules:
//  1. Strip trailing .git suffix
//  2. Lowercase the result
//  3. Strip leading https:// or http://
//  4. Convert SSH user@host:path to host/path
func normalizeRemoteURL(u string) string {
	u = strings.TrimSpace(u)

	// Handle SSH form: git@github.com:clay/my-project.git
	if idx := strings.Index(u, "@"); idx != -1 {
		// Strip everything up to and including the "@".
		rest := u[idx+1:]
		// Replace the first ":" (host/path separator) with "/".
		rest = strings.Replace(rest, ":", "/", 1)
		u = rest
	} else {
		// Strip leading https:// or http://
		u = strings.TrimPrefix(u, "https://")
		u = strings.TrimPrefix(u, "http://")
	}

	// Strip trailing .git suffix.
	u = strings.TrimSuffix(u, ".git")

	// Lowercase.
	u = strings.ToLower(u)

	return u
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

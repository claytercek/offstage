// Package store manages the sync store — the private git repository that
// backs the offstage sync system.
package store

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Store wraps a local path to the cloned sync store.
type Store struct {
	Path string // local path to the cloned sync store
}

// gitBin returns the path to the git binary, checking the system PATH first
// and falling back to /usr/bin/git.
func gitBin() string {
	if p, err := exec.LookPath("git"); err == nil {
		return p
	}
	return "/usr/bin/git"
}

// Clone clones the sync store from url into localPath.
// If localPath already exists and is non-empty it returns an error rather than
// clobbering the existing clone.
func Clone(url, localPath string) error {
	// Detect existing store.
	info, err := os.Stat(localPath)
	if err == nil && info.IsDir() {
		entries, readErr := os.ReadDir(localPath)
		if readErr == nil && len(entries) > 0 {
			return fmt.Errorf("store already exists at %s; run 'offstage init' only once", localPath)
		}
	}

	if err := os.MkdirAll(localPath, 0o700); err != nil {
		return fmt.Errorf("create store dir: %w", err)
	}

	// Use the system git binary.
	cmd := exec.Command(gitBin(), "clone", url, localPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// Clean up the directory we may have created (or partially populated)
		// so a retry works.
		_ = os.RemoveAll(localPath)
		return fmt.Errorf("git clone %s: %w", url, err)
	}
	return nil
}

// Open verifies that path is a git repository and returns a Store for it.
// Returns an error if path is not a git repository.
func Open(path string) (*Store, error) {
	cmd := exec.Command(gitBin(), "-C", path, "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("not a git repository: %s", path)
	}
	return &Store{Path: path}, nil
}

// Exec runs an arbitrary git command inside the store directory, inheriting
// stdout and stderr from the current process.
func (s *Store) Exec(args ...string) error {
	fullArgs := append([]string{"-C", s.Path}, args...)
	cmd := exec.Command(gitBin(), fullArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

// ExecOutput runs an arbitrary git command inside the store directory and
// captures stdout, returning (output, error).
func (s *Store) ExecOutput(args ...string) (string, error) {
	fullArgs := append([]string{"-C", s.Path}, args...)
	cmd := exec.Command(gitBin(), fullArgs...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return out.String(), nil
}

// CurrentBranch returns the name of the current branch using
// git symbolic-ref --short HEAD.
func (s *Store) CurrentBranch() (string, error) {
	out, err := s.ExecOutput("symbolic-ref", "--short", "HEAD")
	if err != nil {
		return "", fmt.Errorf("get current branch: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// BranchExists returns true if the named local branch exists.
func (s *Store) BranchExists(branch string) bool {
	fullArgs := []string{"-C", s.Path, "rev-parse", "--verify", "refs/heads/" + branch}
	cmd := exec.Command(gitBin(), fullArgs...)
	return cmd.Run() == nil
}

// CreateBranch creates a new branch from the current HEAD and checks it out.
func (s *Store) CreateBranch(branch string) error {
	if err := s.Exec("checkout", "-b", branch); err != nil {
		return fmt.Errorf("create branch %s: %w", branch, err)
	}
	return nil
}

// Checkout switches to the named branch.
func (s *Store) Checkout(branch string) error {
	if err := s.Exec("checkout", branch); err != nil {
		return fmt.Errorf("checkout %s: %w", branch, err)
	}
	return nil
}

// Add stages the given paths in the store.
func (s *Store) Add(paths ...string) error {
	args := append([]string{"add"}, paths...)
	if err := s.Exec(args...); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	return nil
}

// Commit creates a commit with the given message, allowing empty commits.
func (s *Store) Commit(message string) error {
	if err := s.Exec("commit", "--allow-empty", "-m", message); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}
	return nil
}

// Push pushes the current HEAD to origin.
func (s *Store) Push() error {
	if err := s.Exec("push", "origin", "HEAD"); err != nil {
		return fmt.Errorf("git push: %w", err)
	}
	return nil
}

// Fetch fetches from origin.
func (s *Store) Fetch() error {
	if err := s.Exec("fetch", "origin"); err != nil {
		return fmt.Errorf("git fetch: %w", err)
	}
	return nil
}

// Pull performs a fast-forward-only pull.
func (s *Store) Pull() error {
	if err := s.Exec("pull", "--ff-only"); err != nil {
		return fmt.Errorf("git pull: %w", err)
	}
	return nil
}

// MergeBranch merges the named branch into HEAD using --no-ff.
// If the merge fails (e.g. conflict), it returns an error with conflict info.
func (s *Store) MergeBranch(branch string) error {
	fullArgs := []string{"-C", s.Path, "merge", "--no-ff", branch}
	cmd := exec.Command(gitBin(), fullArgs...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git merge %s: %w\n%s", branch, err, out.String())
	}
	return nil
}

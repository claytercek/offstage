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

// BranchState describes the relationship between a local branch and its
// origin remote tracking counterpart.
type BranchState int

const (
	// BranchNotFound means neither the local branch nor the remote tracking
	// branch exist.
	BranchNotFound BranchState = iota

	// BranchLocalOnly means the local branch exists but has no remote counterpart.
	BranchLocalOnly

	// BranchRemoteOnly means the remote tracking branch exists but no local
	// branch has been created for it yet.
	BranchRemoteOnly

	// BranchSynced means both local and remote exist and local is at or ahead
	// of remote (no new remote commits to pull).
	BranchSynced

	// BranchRemoteAhead means both local and remote exist, the remote has new
	// commits, and local does not (safe to fast-forward).
	BranchRemoteAhead

	// BranchDiverged means both local and remote have commits the other does
	// not, i.e. a fork has occurred.
	BranchDiverged
)

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

// git runs an arbitrary git command inside the store directory, inheriting
// stdout and stderr from the current process.
func (s *Store) git(args ...string) error {
	fullArgs := append([]string{"-C", s.Path}, args...)
	cmd := exec.Command(gitBin(), fullArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

// gitOutput runs an arbitrary git command inside the store directory and
// captures stdout, returning (output, error).
func (s *Store) gitOutput(args ...string) (string, error) {
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
	out, err := s.gitOutput("symbolic-ref", "--short", "HEAD")
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

// RemoteBranchExists returns true if the named remote tracking branch exists
// (i.e. refs/remotes/origin/<branch>).
func (s *Store) RemoteBranchExists(branch string) bool {
	fullArgs := []string{"-C", s.Path, "rev-parse", "--verify", "refs/remotes/origin/" + branch}
	cmd := exec.Command(gitBin(), fullArgs...)
	return cmd.Run() == nil
}

// BranchSyncState inspects the local branch and its origin remote tracking
// branch and returns a BranchState constant describing their relationship.
// It must be called after a Fetch so that remote tracking refs are current.
func (s *Store) BranchSyncState(branch string) (BranchState, error) {
	localExists := s.BranchExists(branch)
	remoteExists := s.RemoteBranchExists(branch)

	switch {
	case !localExists && !remoteExists:
		return BranchNotFound, nil

	case !localExists && remoteExists:
		return BranchRemoteOnly, nil

	case localExists && !remoteExists:
		return BranchLocalOnly, nil

	default: // both exist
		localRef := "refs/heads/" + branch
		remoteRef := "refs/remotes/origin/" + branch

		remoteAheadOut, err := s.gitOutput("log", localRef+".."+remoteRef, "--oneline")
		if err != nil {
			return 0, fmt.Errorf("check remote commits for %s: %w", branch, err)
		}
		localAheadOut, err := s.gitOutput("log", remoteRef+".."+localRef, "--oneline")
		if err != nil {
			return 0, fmt.Errorf("check local commits for %s: %w", branch, err)
		}

		remoteHasNew := strings.TrimSpace(remoteAheadOut) != ""
		localHasNew := strings.TrimSpace(localAheadOut) != ""

		if remoteHasNew && localHasNew {
			return BranchDiverged, nil
		}
		if remoteHasNew {
			return BranchRemoteAhead, nil
		}
		return BranchSynced, nil
	}
}

// CreateBranch creates a new branch from the current HEAD and checks it out.
func (s *Store) CreateBranch(branch string) error {
	if err := s.git("checkout", "-b", branch); err != nil {
		return fmt.Errorf("create branch %s: %w", branch, err)
	}
	return nil
}

// CheckoutTrackRemote creates a local branch that tracks origin/<branch>
// and checks it out.
func (s *Store) CheckoutTrackRemote(branch string) error {
	if err := s.git("checkout", "-b", branch, "origin/"+branch); err != nil {
		return fmt.Errorf("checkout remote branch %s: %w", branch, err)
	}
	return nil
}

// Checkout switches to the named branch.
func (s *Store) Checkout(branch string) error {
	if err := s.git("checkout", branch); err != nil {
		return fmt.Errorf("checkout %s: %w", branch, err)
	}
	return nil
}

// StageAll stages all changes in the store working tree, using --force to
// override any global gitignore rules.
func (s *Store) StageAll() error {
	if err := s.git("add", "--force", "."); err != nil {
		return fmt.Errorf("stage all: %w", err)
	}
	return nil
}

// StageFile stages a specific file within the store, using --force to
// override any global gitignore rules.
func (s *Store) StageFile(path string) error {
	if err := s.git("add", "--force", path); err != nil {
		return fmt.Errorf("stage file %s: %w", path, err)
	}
	return nil
}

// StagedFiles returns the newline-separated list of files staged for the next
// commit (output of git diff --staged --name-only).
func (s *Store) StagedFiles() (string, error) {
	out, err := s.gitOutput("diff", "--staged", "--name-only")
	if err != nil {
		return "", fmt.Errorf("list staged files: %w", err)
	}
	return out, nil
}

// ConflictedFiles returns the newline-separated list of files with unresolved
// merge conflicts (output of git diff --name-only --diff-filter=U).
func (s *Store) ConflictedFiles() (string, error) {
	out, err := s.gitOutput("diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return "", fmt.Errorf("list conflicted files: %w", err)
	}
	return out, nil
}

// Add stages the given paths in the store.
func (s *Store) Add(paths ...string) error {
	args := append([]string{"add"}, paths...)
	if err := s.git(args...); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	return nil
}

// Commit creates a commit with the given message, allowing empty commits.
func (s *Store) Commit(message string) error {
	if err := s.git("commit", "--allow-empty", "-m", message); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}
	return nil
}

// Push pushes the current HEAD to origin.
func (s *Store) Push() error {
	if err := s.git("push", "origin", "HEAD"); err != nil {
		return fmt.Errorf("git push: %w", err)
	}
	return nil
}

// Fetch fetches from origin.
func (s *Store) Fetch() error {
	if err := s.git("fetch", "origin"); err != nil {
		return fmt.Errorf("git fetch: %w", err)
	}
	return nil
}

// Pull performs a fast-forward-only pull.
func (s *Store) Pull() error {
	if err := s.git("pull", "--ff-only"); err != nil {
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

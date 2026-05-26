package hooks_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/claytercek/offstage/internal/hooks"
)

// FakeExecutor records calls and returns configured outcomes for unit testing.
type FakeExecutor struct {
	// PullDelay simulates a slow Pull (for timeout tests).
	PullDelay time.Duration
	// PullErr is returned by Pull (nil means success).
	PullErr error
	// PushDelay simulates a slow Push.
	PushDelay time.Duration
	// PushErr is returned by Push.
	PushErr error

	// Recorded calls.
	PullCalled bool
	PushCalled bool
}

func (f *FakeExecutor) Pull(projectDir, projectID, branch string) error {
	f.PullCalled = true
	if f.PullDelay > 0 {
		time.Sleep(f.PullDelay)
	}
	return f.PullErr
}

func (f *FakeExecutor) Push(projectDir string, include, exclude []string, projectID, branch string) error {
	f.PushCalled = true
	if f.PushDelay > 0 {
		time.Sleep(f.PushDelay)
	}
	return f.PushErr
}

// TestWarnNotBlockPostCheckout verifies ADR-0002: a Pull error produces a
// warning on stderr but RunWithExecutor still returns nil (warn-not-block).
func TestWarnNotBlockPostCheckout(t *testing.T) {
	repoDir := t.TempDir()
	mustRunGit(t, repoDir, "init", "-b", "main")
	mustRunGit(t, repoDir, "config", "user.email", "test@example.com")
	mustRunGit(t, repoDir, "config", "user.name", "Test")
	mustRunGit(t, repoDir, "remote", "add", "origin", "https://github.com/test/repo.git")
	touchRunFile(t, repoDir, "README.md")
	mustRunGit(t, repoDir, "add", ".")
	mustRunGit(t, repoDir, "commit", "-m", "init")
	chdir(t, repoDir)

	cfgTmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgTmp)

	fake := &FakeExecutor{PullErr: errors.New("simulated pull failure")}

	stderr, runErr := captureStderr(t, func() error {
		return hooks.RunWithExecutor("post-checkout", []string{"abc", "def", "1"}, fake)
	})

	if runErr != nil {
		t.Errorf("expected nil (warn-not-block), got %v", runErr)
	}
	if !fake.PullCalled {
		t.Error("expected Pull to be called")
	}
	if !strings.Contains(stderr, "warning") {
		t.Errorf("expected a warning on stderr, got: %q", stderr)
	}
}

// TestWarnNotBlockPrePush verifies ADR-0002: a Push error produces a warning
// but RunWithExecutor returns nil.
func TestWarnNotBlockPrePush(t *testing.T) {
	repoDir := t.TempDir()
	mustRunGit(t, repoDir, "init", "-b", "main")
	mustRunGit(t, repoDir, "config", "user.email", "test@example.com")
	mustRunGit(t, repoDir, "config", "user.name", "Test")
	mustRunGit(t, repoDir, "remote", "add", "origin", "https://github.com/test/repo.git")
	touchRunFile(t, repoDir, "README.md")
	mustRunGit(t, repoDir, "add", ".")
	mustRunGit(t, repoDir, "commit", "-m", "init")
	writeRunManifest(t, repoDir, `include = ["README.md"]`)
	chdir(t, repoDir)

	cfgTmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgTmp)

	fake := &FakeExecutor{PushErr: errors.New("simulated push failure")}

	stderr, runErr := captureStderr(t, func() error {
		return hooks.RunWithExecutor("pre-push", nil, fake)
	})

	if runErr != nil {
		t.Errorf("expected nil (warn-not-block), got %v", runErr)
	}
	if !fake.PushCalled {
		t.Error("expected Push to be called")
	}
	if !strings.Contains(stderr, "warning") {
		t.Errorf("expected a warning on stderr, got: %q", stderr)
	}
}

// TestTimeoutPostCheckout verifies that a slow Pull is interrupted by the
// configured hook timeout and produces a timeout warning.
func TestTimeoutPostCheckout(t *testing.T) {
	repoDir := t.TempDir()
	mustRunGit(t, repoDir, "init", "-b", "main")
	mustRunGit(t, repoDir, "config", "user.email", "test@example.com")
	mustRunGit(t, repoDir, "config", "user.name", "Test")
	mustRunGit(t, repoDir, "remote", "add", "origin", "https://github.com/test/repo.git")
	touchRunFile(t, repoDir, "README.md")
	mustRunGit(t, repoDir, "add", ".")
	mustRunGit(t, repoDir, "commit", "-m", "init")
	chdir(t, repoDir)

	cfgTmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgTmp)

	// PullDelay exceeds timeout so the hook should time out.
	fake := &FakeExecutor{PullDelay: 2 * time.Second}

	stderr, runErr := captureStderr(t, func() error {
		return hooks.RunWithExecutor(
			"post-checkout",
			[]string{"abc", "def", "1"},
			fake,
			hooks.WithTimeout(100*time.Millisecond),
		)
	})

	if runErr != nil {
		t.Errorf("expected nil (warn-not-block), got %v", runErr)
	}
	if !strings.Contains(stderr, "timed out") {
		t.Errorf("expected 'timed out' in stderr, got: %q", stderr)
	}
}

// TestTimeoutPrePush verifies that a slow Push is interrupted by the
// configured hook timeout and produces a timeout warning.
func TestTimeoutPrePush(t *testing.T) {
	repoDir := t.TempDir()
	mustRunGit(t, repoDir, "init", "-b", "main")
	mustRunGit(t, repoDir, "config", "user.email", "test@example.com")
	mustRunGit(t, repoDir, "config", "user.name", "Test")
	mustRunGit(t, repoDir, "remote", "add", "origin", "https://github.com/test/repo.git")
	touchRunFile(t, repoDir, "README.md")
	mustRunGit(t, repoDir, "add", ".")
	mustRunGit(t, repoDir, "commit", "-m", "init")
	writeRunManifest(t, repoDir, `include = ["README.md"]`)
	chdir(t, repoDir)

	cfgTmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgTmp)

	fake := &FakeExecutor{PushDelay: 2 * time.Second}

	stderr, runErr := captureStderr(t, func() error {
		return hooks.RunWithExecutor(
			"pre-push",
			nil,
			fake,
			hooks.WithTimeout(100*time.Millisecond),
		)
	})

	if runErr != nil {
		t.Errorf("expected nil (warn-not-block), got %v", runErr)
	}
	if !strings.Contains(stderr, "timed out") {
		t.Errorf("expected 'timed out' in stderr, got: %q", stderr)
	}
}

// TestSilentNoOpNotInitialized verifies that when offstage is not configured,
// Run returns nil silently without any stderr output (no executor wired in the
// live path when config is absent).
func TestSilentNoOpNotInitialized(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	stderr, runErr := captureStderr(t, func() error {
		// Use Run (not RunWithExecutor) to exercise the live path where config
		// absence triggers a silent no-op.
		return hooks.Run("post-checkout", []string{"abc", "def", "1"})
	})

	if runErr != nil {
		t.Errorf("expected nil, got %v", runErr)
	}
	if stderr != "" {
		t.Errorf("expected no stderr output for silent no-op, got: %q", stderr)
	}
}

// TestSilentNoOpFileCheckout verifies that a file checkout (flag=0) is a
// silent no-op without calling the executor.
func TestSilentNoOpFileCheckout(t *testing.T) {
	fake := &FakeExecutor{}

	stderr, runErr := captureStderr(t, func() error {
		return hooks.RunWithExecutor("post-checkout", []string{"abc", "def", "0"}, fake)
	})

	if runErr != nil {
		t.Errorf("expected nil, got %v", runErr)
	}
	if fake.PullCalled {
		t.Error("expected Pull to not be called for file checkout")
	}
	if stderr != "" {
		t.Errorf("expected no stderr output for file checkout, got: %q", stderr)
	}
}

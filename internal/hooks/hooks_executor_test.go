package hooks_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/claytercek/offstage/internal/hooks"
	"github.com/claytercek/offstage/internal/manifest"
	"github.com/claytercek/offstage/internal/resolver"
	"github.com/claytercek/offstage/internal/syncenv"
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
// WithSyncEnv bypasses live-path initialization so no real git repository or
// on-disk config is required.
func TestWarnNotBlockPostCheckout(t *testing.T) {
	env, _ := fakeEnv(t)
	fake := &FakeExecutor{PullErr: errors.New("simulated pull failure")}

	stderr, runErr := captureStderr(t, func() error {
		return hooks.RunWithExecutor("post-checkout", []string{"abc", "def", "1"}, fake, hooks.WithSyncEnv(env))
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
// but RunWithExecutor returns nil. WithSyncEnv bypasses live-path initialization.
func TestWarnNotBlockPrePush(t *testing.T) {
	env, _ := fakeEnv(t)
	fake := &FakeExecutor{PushErr: errors.New("simulated push failure")}

	stderr, runErr := captureStderr(t, func() error {
		return hooks.RunWithExecutor("pre-push", nil, fake, hooks.WithSyncEnv(env))
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
// WithSyncEnv bypasses live-path initialization so no real git repository is required.
func TestTimeoutPostCheckout(t *testing.T) {
	env, _ := fakeEnv(t)
	// PullDelay exceeds timeout so the hook should time out.
	fake := &FakeExecutor{PullDelay: 2 * time.Second}

	stderr, runErr := captureStderr(t, func() error {
		return hooks.RunWithExecutor(
			"post-checkout",
			[]string{"abc", "def", "1"},
			fake,
			hooks.WithSyncEnv(env),
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
// WithSyncEnv bypasses live-path initialization so no real git repository is required.
func TestTimeoutPrePush(t *testing.T) {
	env, _ := fakeEnv(t)
	fake := &FakeExecutor{PushDelay: 2 * time.Second}

	stderr, runErr := captureStderr(t, func() error {
		return hooks.RunWithExecutor(
			"pre-push",
			nil,
			fake,
			hooks.WithSyncEnv(env),
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

// fakeEnv builds a minimal *syncenv.SyncEnv that satisfies hook env access
// without requiring a real git repository or config on disk. The project dir is
// a temp directory that acts as the repository root.
func fakeEnv(t *testing.T) (*syncenv.SyncEnv, string) {
	t.Helper()
	dir := t.TempDir()
	env := &syncenv.SyncEnv{
		Resolved: &resolver.Result{
			RepoRoot:   dir,
			ProjectID:  "github.com/test/project",
			BranchName: "main",
		},
		Manifest: &manifest.ProjectConfig{
			Include: []string{"CONTEXT.md"},
			Exclude: []string{},
		},
	}
	return env, dir
}


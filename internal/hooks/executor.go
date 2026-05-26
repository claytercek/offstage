package hooks

import (
	"time"

	"github.com/claytercek/offstage/internal/store"
	"github.com/claytercek/offstage/internal/syncer"
)

// HookExecutor is the seam between hook orchestration and sync operations.
// Implementations call the real syncer (LiveExecutor) or record calls for
// unit tests (FakeExecutor in tests).
type HookExecutor interface {
	// Pull fetches the store branch and copies tracked files to projectDir.
	Pull(projectDir, projectID, branch string) error
	// Push collects files from projectDir and pushes them to the store branch.
	Push(projectDir string, include, exclude []string, projectID, branch string) error
}

// LiveExecutor wires HookExecutor to the real syncer functions.
type LiveExecutor struct {
	Store *store.Store
}

func (e *LiveExecutor) Pull(projectDir, projectID, branch string) error {
	return syncer.Pull(e.Store, projectDir, projectID, branch, false)
}

func (e *LiveExecutor) Push(projectDir string, include, exclude []string, projectID, branch string) error {
	return syncer.Push(e.Store, projectDir, include, exclude, projectID, branch, false)
}

// runOptions carries optional overrides for RunWithExecutor.
type runOptions struct {
	timeout time.Duration // 0 means use config value
}

// RunOption is a functional option for RunWithExecutor.
type RunOption func(*runOptions)

// WithTimeout overrides the hook timeout for this invocation.
// Primarily used in tests to avoid waiting for the real config timeout.
func WithTimeout(d time.Duration) RunOption {
	return func(o *runOptions) { o.timeout = d }
}

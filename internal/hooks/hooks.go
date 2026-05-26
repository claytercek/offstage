// Package hooks manages offstage git hooks installation and execution.
package hooks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/claytercek/offstage/internal/config"
	"github.com/claytercek/offstage/internal/gitexclude"
	"github.com/claytercek/offstage/internal/manifest"
	"github.com/claytercek/offstage/internal/syncenv"
	"github.com/claytercek/offstage/internal/syncer"
)

// InstallLocal writes hook scripts to .git/hooks/ of the repo at repoGitDir.
// repoGitDir should be the absolute path to the .git directory.
func InstallLocal(repoGitDir string) error {
	dir := filepath.Join(repoGitDir, "hooks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create .git/hooks dir: %w", err)
	}

	for _, hookName := range []string{"post-checkout", "pre-push"} {
		script := fmt.Sprintf("#!/bin/sh\noffstage hooks run %s \"$@\"\n", hookName)
		p := filepath.Join(dir, hookName)

		existing, err := os.ReadFile(p)
		if err == nil {
			if strings.Contains(string(existing), "offstage hooks run") {
				fmt.Println("hooks already installed")
				return nil
			}
			return fmt.Errorf("hook %q already exists and was not written by offstage; remove it manually first", hookName)
		}

		if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
			return fmt.Errorf("write %s hook: %w", hookName, err)
		}
	}

	fmt.Printf("hooks installed locally in %s\n", dir)
	return nil
}

// UninstallLocal removes offstage-written hook scripts from .git/hooks/.
// Other hook files are left untouched. Global core.hooksPath is not modified.
func UninstallLocal(repoGitDir string) error {
	dir := filepath.Join(repoGitDir, "hooks")
	removed := 0

	for _, hookName := range []string{"post-checkout", "pre-push"} {
		p := filepath.Join(dir, hookName)
		content, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if !strings.Contains(string(content), "offstage hooks run") {
			continue
		}
		if err := os.Remove(p); err != nil {
			return fmt.Errorf("remove %s hook: %w", hookName, err)
		}
		removed++
	}

	if removed == 0 {
		fmt.Println("nothing to uninstall")
	} else {
		fmt.Println("hooks uninstalled")
	}
	return nil
}

// Run is the entry point called by hook shell scripts. It never returns a
// non-nil error — any failure is printed as a warning and the hook exits 0.
func Run(hookName string, args []string) error {
	return RunWithExecutor(hookName, args, nil)
}

// RunWithExecutor is the injectable variant of Run. Pass a non-nil executor
// to override the real syncer calls (used in unit tests). Pass RunOption
// values to override config-driven defaults such as the timeout.
func RunWithExecutor(hookName string, args []string, ex HookExecutor, opts ...RunOption) error {
	switch hookName {
	case "post-checkout":
		return runPostCheckout(args, ex, opts)
	case "pre-push":
		return runPrePush(ex, opts)
	default:
		return nil
	}
}

// openEnv returns the SyncEnv to use for the hook. If an env was injected via
// WithSyncEnv it is returned directly. Otherwise the live path is taken:
// syncenv.Open is called and the configured timeout is applied to opts.
//
// The hookName parameter is used only for warning messages on the live path.
// Returns (nil, nil) when the hook should silently no-op (offstage not
// initialized). Returns (nil, error) for unexpected initialization failures
// that should produce a warning.
func openEnv(cwd, hookName string, opts []RunOption, needManifest bool) (*syncenv.SyncEnv, []RunOption, error) {
	o := &runOptions{}
	for _, opt := range opts {
		opt(o)
	}

	// If a pre-built env was injected (e.g. by tests), use it directly.
	if o.env != nil {
		return o.env, opts, nil
	}

	// Live path: use syncenv.Open which encapsulates config + resolver + store.
	var openOpts []syncenv.Option
	if needManifest {
		openOpts = append(openOpts, syncenv.WithManifest())
	}

	env, err := syncenv.Open(cwd, openOpts...)
	if err != nil {
		if errors.Is(err, config.ErrNotInitialized) {
			return nil, opts, nil // silent no-op
		}
		return nil, opts, fmt.Errorf("offstage hooks %s: %w", hookName, err)
	}

	// Pick up the configured timeout from config when not explicitly set.
	if !hasTimeoutOpt(opts) {
		opts = append(opts, WithTimeout(time.Duration(env.Config.Hooks.Timeout())*time.Second))
	}

	return env, opts, nil
}

// runWithTimeout runs op in a goroutine, returning nil if it succeeds or prints
// a warning and returns nil if it times out or returns a non-fatal error.
// This is the single shared implementation of the timeout-and-degrade pattern.
func runWithTimeout(hookName string, timeout time.Duration, op func() error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- op()
	}()

	select {
	case err := <-done:
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: offstage hooks %s: %v\n", hookName, err)
		}
	case <-ctx.Done():
		fmt.Fprintf(os.Stderr, "warning: offstage hooks %s: timed out after %v\n", hookName, timeout)
	}
}

// runPostCheckout implements the post-checkout hook. Git calls it with:
// <prev-HEAD> <new-HEAD> <flag> where flag is 1 for branch switch, 0 for file checkout.
func runPostCheckout(args []string, ex HookExecutor, opts []RunOption) error {
	if len(args) >= 3 && args[2] == "0" {
		return nil // file checkout, not a branch switch
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}

	env, opts, err := openEnv(cwd, "post-checkout", opts, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		return nil
	}
	if env == nil {
		return nil // silent no-op (not initialized)
	}

	if strings.HasPrefix(env.Resolved.BranchName, "detached/") {
		return nil
	}

	// Wire live executor from env if none was injected.
	if ex == nil {
		ex = &LiveExecutor{Store: env.Store}
	}

	repoRoot := env.Resolved.RepoRoot
	projectID := env.Resolved.ProjectID
	branch := env.Resolved.BranchName
	timeout := effectiveTimeout(opts)

	runWithTimeout("post-checkout", timeout, func() error {
		err := ex.Pull(repoRoot, projectID, branch)
		if errors.Is(err, syncer.ErrBranchNotFound) {
			return nil // new branch, no store branch yet
		}
		return err
	})

	return nil
}

// runPrePush implements the pre-push hook. Any error is printed as a warning;
// the push always proceeds (always exits 0).
func runPrePush(ex HookExecutor, opts []RunOption) error {
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}

	env, opts, err := openEnv(cwd, "pre-push", opts, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		return nil
	}
	if env == nil {
		return nil // silent no-op (not initialized)
	}

	// Wire live executor from env if none was injected.
	if ex == nil {
		ex = &LiveExecutor{Store: env.Store}
	}

	mf := env.Manifest
	if mf == nil {
		// Manifest not loaded (WithManifest not requested — shouldn't happen here,
		// but guard defensively).
		mf = &manifest.ProjectConfig{}
	}

	repoRoot := env.Resolved.RepoRoot
	projectID := env.Resolved.ProjectID
	branch := env.Resolved.BranchName
	timeout := effectiveTimeout(opts)

	if mf.GitExclude.AutoSync {
		if _, err := gitexclude.Sync(repoRoot, mf.Include); err != nil {
			fmt.Fprintf(os.Stderr, "warning: offstage hooks pre-push: %v\n", err)
		}
	}

	runWithTimeout("pre-push", timeout, func() error {
		return ex.Push(repoRoot, mf.Include, mf.Exclude, projectID, branch)
	})

	return nil
}

// effectiveTimeout returns the timeout from opts, or a sensible default if
// none was set. The caller is responsible for prepending a WithTimeout option
// from config when wiring the live path.
func effectiveTimeout(opts []RunOption) time.Duration {
	o := &runOptions{}
	for _, opt := range opts {
		opt(o)
	}
	if o.timeout > 0 {
		return o.timeout
	}
	// Fallback default when called without a configured timeout (e.g. in tests
	// that inject an executor but omit WithTimeout).
	return 5 * time.Second
}

// hasTimeoutOpt returns true if any of opts sets a timeout.
func hasTimeoutOpt(opts []RunOption) bool {
	o := &runOptions{}
	for _, opt := range opts {
		opt(o)
	}
	return o.timeout > 0
}

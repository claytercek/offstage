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
	"github.com/claytercek/offstage/internal/manifest"
	"github.com/claytercek/offstage/internal/resolver"
	"github.com/claytercek/offstage/internal/store"
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
	switch hookName {
	case "post-checkout":
		return runPostCheckout(args)
	case "pre-push":
		return runPrePush()
	default:
		return nil
	}
}

// runPostCheckout implements the post-checkout hook. Git calls it with:
// <prev-HEAD> <new-HEAD> <flag> where flag is 1 for branch switch, 0 for file checkout.
func runPostCheckout(args []string) error {
	if len(args) >= 3 && args[2] == "0" {
		return nil // file checkout, not a branch switch
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}

	cfg, err := config.Load()
	if errors.Is(err, config.ErrNotInitialized) {
		return nil
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: offstage hooks post-checkout: %v\n", err)
		return nil
	}

	branch, err := resolver.ResolveBranchContext(cwd)
	if err != nil {
		return nil
	}
	if strings.HasPrefix(branch, "detached/") {
		return nil
	}

	projectID, err := resolver.ResolveProjectID(cwd)
	if err != nil {
		return nil
	}

	s, err := store.Open(cfg.StorePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: offstage hooks post-checkout: %v\n", err)
		return nil
	}

	timeout := time.Duration(cfg.Hooks.Timeout()) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- syncer.Pull(s, cwd, projectID, branch, false)
	}()

	select {
	case err := <-done:
		if err != nil {
			if errors.Is(err, syncer.ErrBranchNotFound) {
				return nil // new branch, no store branch yet
			}
			fmt.Fprintf(os.Stderr, "warning: offstage hooks post-checkout: %v\n", err)
		}
	case <-ctx.Done():
		fmt.Fprintf(os.Stderr, "warning: offstage hooks post-checkout: timed out after %v\n", timeout)
	}

	return nil
}

// runPrePush implements the pre-push hook. Any error is printed as a warning;
// the push always proceeds (always exits 0).
func runPrePush() error {
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}

	cfg, err := config.Load()
	if errors.Is(err, config.ErrNotInitialized) {
		return nil
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: offstage hooks pre-push: %v\n", err)
		return nil
	}

	mf, err := manifest.Load(cwd)
	if err != nil {
		return nil // no manifest, nothing to push
	}

	branch, err := resolver.ResolveBranchContext(cwd)
	if err != nil {
		return nil
	}

	projectID, err := resolver.ResolveProjectID(cwd)
	if err != nil {
		return nil
	}

	s, err := store.Open(cfg.StorePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: offstage hooks pre-push: %v\n", err)
		return nil
	}

	timeout := time.Duration(cfg.Hooks.Timeout()) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- syncer.Push(s, cwd, mf.Include, mf.Exclude, projectID, branch, false)
	}()

	select {
	case err := <-done:
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: offstage hooks pre-push: %v\n", err)
		}
	case <-ctx.Done():
		fmt.Fprintf(os.Stderr, "warning: offstage hooks pre-push: timed out after %v\n", timeout)
	}

	return nil
}


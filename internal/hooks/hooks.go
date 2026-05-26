// Package hooks manages offstage git hooks installation and execution.
package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// hooksDir returns the global hooks directory path.
func hooksDir(cfgDir string) string {
	return filepath.Join(cfgDir, "hooks")
}

// Install writes post-checkout and pre-push scripts to the global hooks dir
// and sets core.hooksPath via git config --global.
func Install(cfgDir string) error {
	dir := hooksDir(cfgDir)

	// Check current core.hooksPath
	current, err := gitConfigGet("core.hooksPath")
	if err == nil && current != "" {
		if current == dir {
			fmt.Println("hooks already installed")
			return nil
		}
		return fmt.Errorf("core.hooksPath is already set to %q; unset it first or use --local", current)
	}

	// Create hooks dir
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}

	// Write scripts
	for _, hookName := range []string{"post-checkout", "pre-push"} {
		script := fmt.Sprintf("#!/bin/sh\noffstage hooks run %s \"$@\"\n", hookName)
		p := filepath.Join(dir, hookName)
		if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
			return fmt.Errorf("write %s hook: %w", hookName, err)
		}
	}

	// Set core.hooksPath
	if err := gitConfigSet("core.hooksPath", dir); err != nil {
		return fmt.Errorf("set core.hooksPath: %w", err)
	}

	fmt.Printf("hooks installed: %s\n", dir)
	return nil
}

// Uninstall removes the global hooks dir and unsets core.hooksPath.
func Uninstall(cfgDir string) error {
	dir := hooksDir(cfgDir)

	current, err := gitConfigGet("core.hooksPath")
	if err != nil || current != dir {
		fmt.Println("nothing to uninstall")
		return nil
	}

	if err := gitConfigUnset("core.hooksPath"); err != nil {
		return fmt.Errorf("unset core.hooksPath: %w", err)
	}

	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove hooks dir: %w", err)
	}

	fmt.Println("hooks uninstalled")
	return nil
}

// Run is the entry point called by hook shell scripts.
// This is a stub that always exits 0; real logic is added in later slices.
func Run(hookName string, args []string) error {
	return nil
}

func gitConfigGet(key string) (string, error) {
	out, err := exec.Command("git", "config", "--global", key).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitConfigSet(key, value string) error {
	return exec.Command("git", "config", "--global", key, value).Run()
}

func gitConfigUnset(key string) error {
	return exec.Command("git", "config", "--global", "--unset", key).Run()
}

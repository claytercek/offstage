// Package store manages the sync store — the private git repository that
// backs the offstage sync system.
package store

import (
	"fmt"
	"os"
	"os/exec"
)

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
	cmd := exec.Command("git", "clone", url, localPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// Clean up the empty directory we may have created so a retry works.
		_ = os.Remove(localPath)
		return fmt.Errorf("git clone %s: %w", url, err)
	}
	return nil
}

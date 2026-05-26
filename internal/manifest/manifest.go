// Package manifest tracks which files are managed by offstage for a project.
package manifest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const filename = ".offstagerc.toml"

// ProjectConfig holds per-project sync configuration stored in .offstagerc.toml.
type ProjectConfig struct {
	// Name is an optional project name override. When empty, offstage falls
	// back to normalizing the git remote URL.
	Name string `toml:"name"`

	// Include lists git-ignore-compatible patterns of files that should be synced.
	Include []string `toml:"include"`

	// Exclude lists git-ignore-compatible patterns that should be excluded from syncing.
	Exclude []string `toml:"exclude"`

	// GitExclude controls optional synchronization with git's local exclude file.
	GitExclude GitExcludeConfig `toml:"git_exclude"`
}

type GitExcludeConfig struct {
	// AutoSync keeps git's local exclude file aligned during project commands.
	AutoSync bool `toml:"auto_sync"`
}

// DefaultPatterns returns the default include patterns for new manifests.
func DefaultPatterns() []string {
	return []string{}
}

// Load reads and parses the .offstagerc.toml in dir. If the file does not
// exist, a default ProjectConfig is returned without error. If the file
// exists but cannot be parsed, an error is returned.
func Load(dir string) (cfg *ProjectConfig, err error) {
	p := filepath.Join(dir, filename)
	f, err := os.Open(p)
	if errors.Is(err, os.ErrNotExist) {
		return &ProjectConfig{
			Include: DefaultPatterns(),
			Exclude: []string{},
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", filename, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			cfg = nil
			err = errors.Join(err, fmt.Errorf("close %s: %w", filename, closeErr))
		}
	}()

	var parsed ProjectConfig
	if _, err := toml.NewDecoder(f).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("parse %s: %w", filename, err)
	}
	return &parsed, nil
}

// Write creates or overwrites the .offstagerc.toml in dir with cfg.
func Write(dir string, cfg *ProjectConfig) (err error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	p := filepath.Join(dir, filename)
	f, err := os.Create(p)
	if err != nil {
		return fmt.Errorf("create %s: %w", filename, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close %s: %w", filename, closeErr))
		}
	}()

	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		return fmt.Errorf("write %s: %w", filename, err)
	}
	return nil
}

// AddPattern adds pattern to cfg.Include if it is not already present.
// Returns true if the pattern was added, false if it was already present.
func AddPattern(cfg *ProjectConfig, pattern string) bool {
	for _, p := range cfg.Include {
		if p == pattern {
			return false
		}
	}
	cfg.Include = append(cfg.Include, pattern)
	return true
}

// RemovePattern removes pattern from cfg.Include if it is present.
// Returns true if the pattern was removed, false if it was not found.
func RemovePattern(cfg *ProjectConfig, pattern string) bool {
	for i, p := range cfg.Include {
		if p == pattern {
			cfg.Include = append(cfg.Include[:i], cfg.Include[i+1:]...)
			return true
		}
	}
	return false
}

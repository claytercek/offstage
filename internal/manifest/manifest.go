// Package manifest tracks which files are managed by offstage for a given
// project (via the sync config) and for global state.
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

	// Include lists glob patterns of files that should be synced.
	Include []string `toml:"include"`

	// Exclude lists glob patterns that should be excluded from syncing.
	Exclude []string `toml:"exclude"`
}

// DefaultPatterns returns the default set of include patterns applied when
// .offstagerc.toml is missing.
func DefaultPatterns() []string {
	return []string{
		"CONTEXT.md",
		"docs/adr/**",
		"AGENTS.md",
		".agents/**",
		".offstagerc.toml",
	}
}

// Load reads and parses the .offstagerc.toml in dir. If the file does not
// exist, a default ProjectConfig is returned without error. If the file
// exists but cannot be parsed, an error is returned.
func Load(dir string) (*ProjectConfig, error) {
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
	defer f.Close()

	var cfg ProjectConfig
	if _, err := toml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", filename, err)
	}
	return &cfg, nil
}

// Write creates or overwrites the .offstagerc.toml in dir with cfg.
func Write(dir string, cfg *ProjectConfig) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	p := filepath.Join(dir, filename)
	f, err := os.Create(p)
	if err != nil {
		return fmt.Errorf("create %s: %w", filename, err)
	}
	defer f.Close()
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

// Package config manages the global offstage configuration file at
// ~/.config/offstage/config.toml.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// GlobalConfig holds configuration for globally-tracked home-directory files.
type GlobalConfig struct {
	// Include lists glob patterns of globally-tracked files (relative to $HOME).
	Include []string `toml:"include"`
	// HomeDir overrides $HOME for global file paths (mainly for testing).
	HomeDir string `toml:"home_dir,omitempty"`
}

// Config holds the global offstage configuration.
type Config struct {
	// StoreURL is the git URL of the sync store repository.
	StoreURL string `toml:"store_url"`
	// StorePath is the local path where the sync store is cloned.
	StorePath string `toml:"store_path"`
	// Global holds configuration for globally-tracked home-directory files.
	Global GlobalConfig `toml:"global"`
}

// DefaultGlobalPatterns returns the default list of glob patterns for
// globally-tracked files, relative to $HOME.
func DefaultGlobalPatterns() []string {
	return []string{
		".claude/CLAUDE.md",
		".config/offstage/config.toml",
	}
}

// Dir returns the configuration directory (~/.config/offstage).
func Dir() (string, error) {
	cfgHome, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config dir: %w", err)
	}
	return filepath.Join(cfgHome, "offstage"), nil
}

// Path returns the path to the config file (~/.config/offstage/config.toml).
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

// DefaultStorePath returns the default local path for the sync store
// (~/.local/share/offstage/store).
func DefaultStorePath() (string, error) {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, "offstage", "store"), nil
}

// Load reads and parses the config file. Returns ErrNotInitialized if the
// file does not exist.
var ErrNotInitialized = errors.New("offstage not initialized: run 'offstage init <git-url>'")

func Load() (*Config, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotInitialized
	}
	if err != nil {
		return nil, fmt.Errorf("open config: %w", err)
	}
	defer f.Close()

	var cfg Config
	if _, err := toml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

// Write creates (or overwrites) the config file with the given Config.
func Write(cfg *Config) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	p := filepath.Join(dir, "config.toml")
	f, err := os.Create(p)
	if err != nil {
		return fmt.Errorf("create config file: %w", err)
	}
	defer f.Close()
	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

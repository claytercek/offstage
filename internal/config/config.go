// Package config manages the offstage configuration file at
// ~/.config/offstage/config.toml.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// HooksConfig holds configuration for the offstage git hooks.
type HooksConfig struct {
	// TimeoutSeconds is the timeout for hook execution. Defaults to 5 if unset.
	TimeoutSeconds int `toml:"timeout_seconds"`
}

// Timeout returns the effective timeout in seconds, applying the default of 5
// when TimeoutSeconds is not set (zero value).
func (h HooksConfig) Timeout() int {
	if h.TimeoutSeconds <= 0 {
		return 5
	}
	return h.TimeoutSeconds
}

// Config holds the offstage configuration.
type Config struct {
	// StoreURL is the git URL of the sync store repository.
	StoreURL string `toml:"store_url"`
	// StorePath is the local path where the sync store is cloned.
	StorePath string `toml:"store_path"`
	// Hooks holds configuration for the offstage git hooks.
	Hooks HooksConfig `toml:"hooks"`
}

// Dir returns the configuration directory (~/.config/offstage).
func Dir() (string, error) {
	cfgHome := os.Getenv("XDG_CONFIG_HOME")
	if cfgHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		cfgHome = filepath.Join(home, ".config")
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

func Load() (cfg *Config, err error) {
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
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			cfg = nil
			err = errors.Join(err, fmt.Errorf("close config: %w", closeErr))
		}
	}()

	var parsed Config
	if _, err := toml.NewDecoder(f).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &parsed, nil
}

// Write creates (or overwrites) the config file with the given Config.
func Write(cfg *Config) (err error) {
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
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close config file: %w", closeErr))
		}
	}()

	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

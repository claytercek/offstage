package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gastownhall/offstage/internal/config"
)

func TestWriteAndLoad(t *testing.T) {
	// Point XDG dirs at a temp directory so we don't touch real user config.
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	cfg := &config.Config{
		StoreURL:  "git@github.com:example/store.git",
		StorePath: "/tmp/test-store",
	}

	if err := config.Write(cfg); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got.StoreURL != cfg.StoreURL {
		t.Errorf("StoreURL: got %q, want %q", got.StoreURL, cfg.StoreURL)
	}
	if got.StorePath != cfg.StorePath {
		t.Errorf("StorePath: got %q, want %q", got.StorePath, cfg.StorePath)
	}
}

func TestLoadErrNotInitialized(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected ErrNotInitialized, got nil")
	}
	if err != config.ErrNotInitialized {
		t.Fatalf("expected ErrNotInitialized, got %v", err)
	}
}

func TestPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	p, err := config.Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	want := filepath.Join(tmp, "offstage", "config.toml")
	if p != want {
		t.Errorf("Path: got %q, want %q", p, want)
	}
}

func TestDefaultStorePath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)

	p, err := config.DefaultStorePath()
	if err != nil {
		t.Fatalf("DefaultStorePath: %v", err)
	}
	want := filepath.Join(tmp, "offstage", "store")
	if p != want {
		t.Errorf("DefaultStorePath: got %q, want %q", p, want)
	}
}

func TestDefaultStorePathFallback(t *testing.T) {
	// When XDG_DATA_HOME is unset, should use ~/.local/share/offstage/store.
	t.Setenv("XDG_DATA_HOME", "")

	home, _ := os.UserHomeDir()
	p, err := config.DefaultStorePath()
	if err != nil {
		t.Fatalf("DefaultStorePath: %v", err)
	}
	want := filepath.Join(home, ".local", "share", "offstage", "store")
	if p != want {
		t.Errorf("DefaultStorePath fallback: got %q, want %q", p, want)
	}
}

func TestDefaultGlobalPatterns(t *testing.T) {
	patterns := config.DefaultGlobalPatterns()
	if len(patterns) == 0 {
		t.Fatal("expected non-empty default global patterns")
	}
}

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInitAlreadyInitialized checks that a second 'offstage init' prints an
// error instead of clobbering the existing config.
func TestInitAlreadyInitialized(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("XDG_DATA_HOME", tmp)

	// Write a fake config to simulate a previous init.
	cfgDir := filepath.Join(tmp, "offstage")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfgContent := `store_url = "git@github.com:example/store.git"
store_path = "/tmp/fake-store"
`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(cfgContent), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"init", "git@github.com:example/other.git"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error on second init, got nil")
	}
	if !strings.Contains(err.Error(), "already initialized") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestHelpListsSubcommands verifies that --help output mentions all planned
// subcommands.
func TestHelpListsSubcommands(t *testing.T) {
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"--help"})

	// Help always exits with nil.
	_ = rootCmd.Execute()

	output := out.String()
	for _, sub := range []string{"init", "track", "untrack", "push", "pull", "status", "git", "merge"} {
		if !strings.Contains(output, sub) {
			t.Errorf("--help output missing subcommand %q", sub)
		}
	}
}

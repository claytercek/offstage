package manifest_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/claytercek/offstage/internal/manifest"
)

// TestLoadMissingReturnsEmptyConfig verifies that a missing .offstagerc.toml
// returns an empty config without error.
func TestLoadMissingReturnsEmptyConfig(t *testing.T) {
	tmp := t.TempDir()

	cfg, err := manifest.Load(tmp)
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}

	if len(cfg.Include) != 0 {
		t.Errorf("Include: got %v, want empty slice", cfg.Include)
	}
	if len(cfg.Exclude) != 0 {
		t.Errorf("Exclude: got %v, want empty slice", cfg.Exclude)
	}
	if cfg.Name != "" {
		t.Errorf("Name: got %q, want empty", cfg.Name)
	}
}

// TestTrackAddsPatternAndWritesFile verifies that AddPattern + Write persists
// a new pattern to disk.
func TestTrackAddsPatternAndWritesFile(t *testing.T) {
	tmp := t.TempDir()

	cfg, err := manifest.Load(tmp)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	added := manifest.AddPattern(cfg, "custom/notes/**")
	if !added {
		t.Fatal("AddPattern: expected true for new pattern, got false")
	}

	if err := manifest.Write(tmp, cfg); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Reload from disk and verify the pattern is present.
	got, err := manifest.Load(tmp)
	if err != nil {
		t.Fatalf("Load after write: %v", err)
	}

	found := false
	for _, p := range got.Include {
		if p == "custom/notes/**" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("pattern not found after round-trip; Include: %v", got.Include)
	}
}

// TestUntrackRemovesPattern verifies that RemovePattern removes a pattern and
// the change survives a write/load round-trip.
func TestUntrackRemovesPattern(t *testing.T) {
	tmp := t.TempDir()

	cfg, err := manifest.Load(tmp)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	manifest.AddPattern(cfg, "remove-me.md")
	if err := manifest.Write(tmp, cfg); err != nil {
		t.Fatalf("Write: %v", err)
	}

	cfg2, err := manifest.Load(tmp)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	removed := manifest.RemovePattern(cfg2, "remove-me.md")
	if !removed {
		t.Fatal("RemovePattern: expected true, got false")
	}

	if err := manifest.Write(tmp, cfg2); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := manifest.Load(tmp)
	if err != nil {
		t.Fatalf("Load after remove: %v", err)
	}
	for _, p := range got.Include {
		if p == "remove-me.md" {
			t.Errorf("pattern still present after remove; Include: %v", got.Include)
		}
	}
}

// TestUntrackUntrackedPatternReturnsFalse verifies that removing a pattern
// that is not present returns false without error.
func TestUntrackUntrackedPatternReturnsFalse(t *testing.T) {
	tmp := t.TempDir()

	cfg, err := manifest.Load(tmp)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	removed := manifest.RemovePattern(cfg, "does-not-exist.md")
	if removed {
		t.Fatal("RemovePattern: expected false for absent pattern, got true")
	}
}

// TestRoundTripPreservesAllFields verifies that Write/Load preserves Name,
// Include, and Exclude.
func TestRoundTripPreservesAllFields(t *testing.T) {
	tmp := t.TempDir()

	original := &manifest.ProjectConfig{
		Name:    "my-project",
		Include: []string{"CONTEXT.md", "docs/adr/**"},
		Exclude: []string{"docs/adr/draft-*"},
		GitExclude: manifest.GitExcludeConfig{
			AutoSync: true,
		},
	}

	if err := manifest.Write(tmp, original); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := manifest.Load(tmp)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got.Name != original.Name {
		t.Errorf("Name: got %q, want %q", got.Name, original.Name)
	}
	if !reflect.DeepEqual(got.Include, original.Include) {
		t.Errorf("Include: got %v, want %v", got.Include, original.Include)
	}
	if !reflect.DeepEqual(got.Exclude, original.Exclude) {
		t.Errorf("Exclude: got %v, want %v", got.Exclude, original.Exclude)
	}
	if got.GitExclude.AutoSync != original.GitExclude.AutoSync {
		t.Errorf("GitExclude.AutoSync: got %v, want %v", got.GitExclude.AutoSync, original.GitExclude.AutoSync)
	}
}

// TestAddPatternIsIdempotent verifies that adding an already-present pattern
// returns false and does not create a duplicate.
func TestAddPatternIsIdempotent(t *testing.T) {
	tmp := t.TempDir()

	cfg, err := manifest.Load(tmp)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	manifest.AddPattern(cfg, "CONTEXT.md")
	added := manifest.AddPattern(cfg, "CONTEXT.md")
	if added {
		t.Fatal("AddPattern: expected false for already-present pattern, got true")
	}

	count := 0
	for _, p := range cfg.Include {
		if p == "CONTEXT.md" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 occurrence of CONTEXT.md, got %d; Include: %v", count, cfg.Include)
	}
}

// TestLoadBadSyntaxReturnsError verifies that a malformed TOML file returns
// an error.
func TestLoadBadSyntaxReturnsError(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, ".offstagerc.toml")

	if err := os.WriteFile(p, []byte("include = [[[not valid toml"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err := manifest.Load(tmp)
	if err == nil {
		t.Fatal("expected error for bad TOML, got nil")
	}
}

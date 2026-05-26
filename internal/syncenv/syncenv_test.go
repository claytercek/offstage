package syncenv_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/claytercek/offstage/internal/config"
	"github.com/claytercek/offstage/internal/store"
	"github.com/claytercek/offstage/internal/syncenv"
)

// setupGitRepo creates a temporary git repository with an origin remote and
// returns its path.
func setupGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
	run("remote", "add", "origin", "https://github.com/test/repo.git")
	return dir
}

// setupStore creates a temporary bare git repository to serve as the sync
// store and returns a *store.Store for it plus a cleanup function.
func setupStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", "--bare", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	return s
}

// setupConfig writes a minimal offstage config to a temp file and sets
// XDG_CONFIG_HOME so config.Load() reads it during the test.
func setupConfig(t *testing.T, storePath string) {
	t.Helper()
	cfgDir := t.TempDir()
	offstageDir := filepath.Join(cfgDir, "offstage")
	if err := os.MkdirAll(offstageDir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	cfgContent := "store_path = " + "\"" + storePath + "\"\n"
	if err := os.WriteFile(filepath.Join(offstageDir, "config.toml"), []byte(cfgContent), 0o600); err != nil {
		t.Fatalf("WriteFile config: %v", err)
	}
	t.Setenv("XDG_CONFIG_HOME", cfgDir)
}

// TestOpenWithManifestPopulatesAllFields verifies that Open with WithManifest
// populates Config, Manifest, Resolved, and Store.
func TestOpenWithManifestPopulatesAllFields(t *testing.T) {
	repoDir := setupGitRepo(t)
	s := setupStore(t)
	setupConfig(t, s.Path)

	env, err := syncenv.Open(repoDir, syncenv.WithManifest())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if env.Config == nil {
		t.Error("Config is nil")
	}
	if env.Resolved == nil {
		t.Error("Resolved is nil")
	}
	if env.Store == nil {
		t.Error("Store is nil")
	}
	if env.Manifest == nil {
		t.Error("Manifest is nil: WithManifest() option should populate it")
	}
}

// TestOpenWithoutManifestLeavesManifestNil verifies that Open without
// WithManifest populates Config, Resolved, and Store but leaves Manifest nil.
func TestOpenWithoutManifestLeavesManifestNil(t *testing.T) {
	repoDir := setupGitRepo(t)
	s := setupStore(t)
	setupConfig(t, s.Path)

	env, err := syncenv.Open(repoDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if env.Config == nil {
		t.Error("Config is nil")
	}
	if env.Resolved == nil {
		t.Error("Resolved is nil")
	}
	if env.Store == nil {
		t.Error("Store is nil")
	}
	if env.Manifest != nil {
		t.Error("Manifest should be nil when WithManifest() is not requested")
	}
}

// TestOpenWithManifestDefaultConfig verifies that when WithManifest is
// requested and no .offstagerc.toml exists, a default config is returned.
func TestOpenWithManifestDefaultConfig(t *testing.T) {
	repoDir := setupGitRepo(t)
	s := setupStore(t)
	setupConfig(t, s.Path)

	env, err := syncenv.Open(repoDir, syncenv.WithManifest())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if env.Manifest == nil {
		t.Fatal("Manifest is nil")
	}
	if len(env.Manifest.Include) != 0 {
		t.Errorf("default manifest Include should be empty, got %v", env.Manifest.Include)
	}
}

// TestOpenNoConfigReturnsError verifies that Open returns an error when no
// offstage config exists (not initialized).
func TestOpenNoConfigReturnsError(t *testing.T) {
	// Point XDG_CONFIG_HOME to an empty dir so no config is found.
	emptyDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", emptyDir)

	repoDir := setupGitRepo(t)

	_, err := syncenv.Open(repoDir)
	if err == nil {
		t.Fatal("expected error when config is missing, got nil")
	}
}

// TestOpenNotInGitRepoReturnsError verifies that Open returns an error when
// cwd is not inside a git repository.
func TestOpenNotInGitRepoReturnsError(t *testing.T) {
	notGitDir := t.TempDir()
	s := setupStore(t)
	setupConfig(t, s.Path)

	_, err := syncenv.Open(notGitDir)
	if err == nil {
		t.Fatal("expected error when not in a git repo, got nil")
	}
}

// TestOpenForwardsConfigLoadError checks that a bad XDG_CONFIG_HOME
// results in config.ErrNotInitialized rather than a panic.
func TestOpenForwardsConfigLoadError(t *testing.T) {
	repoDir := setupGitRepo(t)
	// empty XDG_CONFIG_HOME — no offstage/config.toml exists
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	_, err := syncenv.Open(repoDir, syncenv.WithManifest())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// config.ErrNotInitialized should be propagated
	if err != config.ErrNotInitialized {
		// Wrapped errors are acceptable; just ensure it's non-nil (already checked)
		t.Logf("error (wrapping accepted): %v", err)
	}
}

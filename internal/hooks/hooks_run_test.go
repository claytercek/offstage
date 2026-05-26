package hooks_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/claytercek/offstage/internal/config"
	"github.com/claytercek/offstage/internal/hooks"
)

func TestRunUnknownHook(t *testing.T) {
	err := hooks.Run("post-commit", nil)
	if err != nil {
		t.Errorf("expected nil for unknown hook, got %v", err)
	}
}

// --- post-checkout ---

func TestRunPostCheckoutFileCheckout(t *testing.T) {
	err := hooks.Run("post-checkout", []string{"abc123", "def456", "0"})
	if err != nil {
		t.Errorf("expected nil for file checkout, got %v", err)
	}
}

func TestRunPostCheckoutNotInitialized(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	err := hooks.Run("post-checkout", []string{"abc123", "def456", "1"})
	if err != nil {
		t.Errorf("expected nil when not initialized, got %v", err)
	}
}

func TestRunPostCheckoutDetachedHead(t *testing.T) {
	repoDir := t.TempDir()
	mustRunGit(t, repoDir, "init", "-b", "main")
	mustRunGit(t, repoDir, "config", "user.email", "test@example.com")
	mustRunGit(t, repoDir, "config", "user.name", "Test")
	f := filepath.Join(repoDir, "README")
	if err := os.WriteFile(f, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRunGit(t, repoDir, "add", ".")
	mustRunGit(t, repoDir, "commit", "-m", "init")
	mustRunGit(t, repoDir, "checkout", "HEAD^0")

	chdir(t, repoDir)

	cfgTmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgTmp)
	t.Setenv("XDG_DATA_HOME", cfgTmp)
	storeTmp := filepath.Join(cfgTmp, "store")
	mustRunGit(t, storeTmp, "init", "--bare")
	if err := config.Write(&config.Config{
		StoreURL:  storeTmp,
		StorePath: storeTmp,
	}); err != nil {
		t.Fatal(err)
	}

	err := hooks.Run("post-checkout", []string{"abc123", "def456", "1"})
	if err != nil {
		t.Errorf("expected nil for detached HEAD, got %v", err)
	}
}

func TestRunPostCheckoutBranchNotFound(t *testing.T) {
	repoDir := t.TempDir()
	mustRunGit(t, repoDir, "init", "-b", "main")
	mustRunGit(t, repoDir, "config", "user.email", "test@example.com")
	mustRunGit(t, repoDir, "config", "user.name", "Test")
	mustRunGit(t, repoDir, "remote", "add", "origin", "git@github.com:test/myproject.git")
	f := filepath.Join(repoDir, "README")
	if err := os.WriteFile(f, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRunGit(t, repoDir, "add", ".")
	mustRunGit(t, repoDir, "commit", "-m", "init")

	cfgTmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgTmp)
	t.Setenv("XDG_DATA_HOME", cfgTmp)

	upstreamStore := filepath.Join(cfgTmp, "upstream-store")
	mustRunGit(t, upstreamStore, "init", "--bare")
	cloneTmp := filepath.Join(cfgTmp, "clone-tmp")
	mustRunGit(t, cloneTmp, "clone", upstreamStore, cloneTmp)
	mustRunGit(t, cloneTmp, "config", "user.email", "test@example.com")
	mustRunGit(t, cloneTmp, "config", "user.name", "Test")
	initFile := filepath.Join(cloneTmp, ".keep")
	if err := os.WriteFile(initFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRunGit(t, cloneTmp, "add", ".")
	mustRunGit(t, cloneTmp, "commit", "-m", "init store")
	mustRunGit(t, cloneTmp, "push", "origin", "HEAD:main")

	localStore := filepath.Join(cfgTmp, "store")
	mustRunGit(t, localStore, "clone", upstreamStore, localStore)
	if err := config.Write(&config.Config{
		StoreURL:  upstreamStore,
		StorePath: localStore,
	}); err != nil {
		t.Fatal(err)
	}

	chdir(t, repoDir)

	err := hooks.Run("post-checkout", []string{"abc123", "def456", "1"})
	if err != nil {
		t.Errorf("expected nil when branch not found in store, got %v", err)
	}
}

// --- pre-push ---

func TestRunPrePushNotInitialized(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	stderr, runErr := captureStderr(t, func() error {
		return hooks.Run("pre-push", nil)
	})

	if runErr != nil {
		t.Errorf("expected nil, got %v", runErr)
	}
	if stderr != "" {
		t.Errorf("expected no stderr output, got: %q", stderr)
	}
}

func TestRunPrePushNoStore(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	cfg := &config.Config{
		StoreURL:  "file:///dev/null",
		StorePath: filepath.Join(tmp, "nonexistent-store"),
	}
	if err := config.Write(cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}

	repoDir := t.TempDir()
	mustRunGit(t, repoDir, "init")
	mustRunGit(t, repoDir, "config", "user.email", "test@test.com")
	mustRunGit(t, repoDir, "config", "user.name", "Test")
	mustRunGit(t, repoDir, "remote", "add", "origin", "https://github.com/test/repo.git")
	touchRunFile(t, repoDir, "README.md")
	mustRunGit(t, repoDir, "add", ".")
	mustRunGit(t, repoDir, "commit", "-m", "init")

	chdir(t, repoDir)

	stderr, runErr := captureStderr(t, func() error {
		return hooks.Run("pre-push", nil)
	})

	if runErr != nil {
		t.Errorf("expected nil, got %v", runErr)
	}
	if !strings.Contains(stderr, "warning") {
		t.Errorf("expected a warning on stderr, got: %q", stderr)
	}
}

func TestRunPrePushSuccess(t *testing.T) {
	tmp := t.TempDir()

	bareDir := filepath.Join(tmp, "bare.git")
	mustRunGit(t, bareDir, "init", "--bare")

	storeDir := filepath.Join(tmp, "store")
	cloneCmd := exec.Command("git", "clone", bareDir, storeDir)
	if out, err := cloneCmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone: %v\n%s", err, out)
	}
	mustRunGit(t, storeDir, "config", "user.email", "test@test.com")
	mustRunGit(t, storeDir, "config", "user.name", "Test")
	touchRunFile(t, storeDir, "README.md")
	mustRunGit(t, storeDir, "add", ".")
	mustRunGit(t, storeDir, "commit", "-m", "init")
	mustRunGit(t, storeDir, "push", "origin", "HEAD")

	t.Setenv("XDG_CONFIG_HOME", tmp)
	cfg := &config.Config{
		StoreURL:  bareDir,
		StorePath: storeDir,
		Hooks:     config.HooksConfig{TimeoutSeconds: 30},
	}
	if err := config.Write(cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}

	projectDir := t.TempDir()
	touchRunFileContent(t, projectDir, "CONTEXT.md", "# Context\n")
	mustRunGit(t, projectDir, "init")
	mustRunGit(t, projectDir, "config", "user.email", "test@test.com")
	mustRunGit(t, projectDir, "config", "user.name", "Test")
	mustRunGit(t, projectDir, "remote", "add", "origin", "https://github.com/test/project.git")
	mustRunGit(t, projectDir, "add", ".")
	mustRunGit(t, projectDir, "commit", "-m", "init")
	writeRunManifest(t, projectDir, `include = ["CONTEXT.md"]`)

	chdir(t, projectDir)

	runErr := hooks.Run("pre-push", nil)
	if runErr != nil {
		t.Errorf("expected nil, got %v", runErr)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func chdir(t *testing.T, dir string) {
	t.Helper()

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})
}

func captureStderr(t *testing.T, fn func() error) (string, error) {
	t.Helper()

	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := r.Close(); err != nil {
			t.Fatalf("close stderr pipe reader: %v", err)
		}
	}()

	os.Stderr = w
	defer func() {
		os.Stderr = old
	}()

	runErr := fn()
	os.Stderr = old

	if err := w.Close(); err != nil {
		t.Fatalf("close stderr pipe writer: %v", err)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	return buf.String(), runErr
}

func mustRunGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
}

func touchRunFile(t *testing.T, dir, name string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
}

func touchRunFileContent(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeRunManifest(t *testing.T, dir, content string) {
	t.Helper()
	p := filepath.Join(dir, ".offstagerc.toml")
	if err := os.WriteFile(p, []byte(fmt.Sprintf("%s\n", content)), 0o644); err != nil {
		t.Fatal(err)
	}
}

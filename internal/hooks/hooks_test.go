package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// readGitConfig reads a key from the temp git config file set by GIT_CONFIG_GLOBAL.
func readGitConfig(t *testing.T, key string) string {
	t.Helper()
	out, err := exec.Command("git", "config", "--global", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func TestInstall(t *testing.T) {
	tmpGitConfig := filepath.Join(t.TempDir(), "gitconfig")
	t.Setenv("GIT_CONFIG_GLOBAL", tmpGitConfig)

	cfgDir := t.TempDir()

	err := Install(cfgDir)
	if err != nil {
		t.Fatalf("Install: unexpected error: %v", err)
	}

	// Check hooks dir exists
	dir := hooksDir(cfgDir)
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("hooks dir does not exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("hooks dir is not a directory")
	}

	// Check scripts exist and are executable
	for _, hookName := range []string{"post-checkout", "pre-push"} {
		p := filepath.Join(dir, hookName)
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatalf("hook script %q does not exist: %v", hookName, err)
		}
		if fi.Mode()&0o111 == 0 {
			t.Errorf("hook script %q is not executable (mode %v)", hookName, fi.Mode())
		}

		content, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read hook script %q: %v", hookName, err)
		}
		want := "offstage hooks run " + hookName
		if !strings.Contains(string(content), want) {
			t.Errorf("hook script %q does not contain %q; got:\n%s", hookName, want, content)
		}
	}

	// Check git config has core.hooksPath set
	got := readGitConfig(t, "core.hooksPath")
	if got != dir {
		t.Errorf("core.hooksPath = %q; want %q", got, dir)
	}
}

func TestInstallIdempotent(t *testing.T) {
	tmpGitConfig := filepath.Join(t.TempDir(), "gitconfig")
	t.Setenv("GIT_CONFIG_GLOBAL", tmpGitConfig)

	cfgDir := t.TempDir()

	// First install
	if err := Install(cfgDir); err != nil {
		t.Fatalf("first Install: unexpected error: %v", err)
	}

	// Second install should succeed and print "hooks already installed"
	if err := Install(cfgDir); err != nil {
		t.Fatalf("second Install: unexpected error: %v", err)
	}
}

func TestInstallConflict(t *testing.T) {
	tmpGitConfig := filepath.Join(t.TempDir(), "gitconfig")
	t.Setenv("GIT_CONFIG_GLOBAL", tmpGitConfig)

	// Set core.hooksPath to a different directory
	otherDir := t.TempDir()
	if err := exec.Command("git", "config", "--global", "core.hooksPath", otherDir).Run(); err != nil {
		t.Fatalf("set core.hooksPath: %v", err)
	}

	cfgDir := t.TempDir()

	err := Install(cfgDir)
	if err == nil {
		t.Fatal("Install: expected error when core.hooksPath is set to a different path, got nil")
	}
	if !strings.Contains(err.Error(), "core.hooksPath is already set") {
		t.Errorf("Install: error %q does not mention core.hooksPath conflict", err.Error())
	}
}

func TestUninstall(t *testing.T) {
	tmpGitConfig := filepath.Join(t.TempDir(), "gitconfig")
	t.Setenv("GIT_CONFIG_GLOBAL", tmpGitConfig)

	cfgDir := t.TempDir()

	// Install first
	if err := Install(cfgDir); err != nil {
		t.Fatalf("Install: unexpected error: %v", err)
	}

	// Uninstall
	if err := Uninstall(cfgDir); err != nil {
		t.Fatalf("Uninstall: unexpected error: %v", err)
	}

	// Hooks dir should be gone
	dir := hooksDir(cfgDir)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("hooks dir still exists after uninstall")
	}

	// core.hooksPath should be unset
	got := readGitConfig(t, "core.hooksPath")
	if got != "" {
		t.Errorf("core.hooksPath = %q after uninstall; want empty", got)
	}
}

func TestUninstallNothing(t *testing.T) {
	tmpGitConfig := filepath.Join(t.TempDir(), "gitconfig")
	t.Setenv("GIT_CONFIG_GLOBAL", tmpGitConfig)

	cfgDir := t.TempDir()

	// Uninstall with nothing installed should succeed
	if err := Uninstall(cfgDir); err != nil {
		t.Fatalf("Uninstall (nothing): unexpected error: %v", err)
	}
}

func TestRunStub(t *testing.T) {
	if err := Run("post-checkout", []string{"abc", "def", "1"}); err != nil {
		t.Errorf("Run: unexpected error: %v", err)
	}
}

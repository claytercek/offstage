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

// --- Local install/uninstall tests ---

func TestInstallLocal(t *testing.T) {
	gitDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(gitDir, "hooks"), 0o755); err != nil {
		t.Fatalf("create hooks dir: %v", err)
	}

	if err := InstallLocal(gitDir); err != nil {
		t.Fatalf("InstallLocal: unexpected error: %v", err)
	}

	hooksDir := filepath.Join(gitDir, "hooks")
	for _, hookName := range []string{"post-checkout", "pre-push"} {
		p := filepath.Join(hooksDir, hookName)
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
}

func TestInstallLocalIdempotent(t *testing.T) {
	gitDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(gitDir, "hooks"), 0o755); err != nil {
		t.Fatalf("create hooks dir: %v", err)
	}

	// First install
	if err := InstallLocal(gitDir); err != nil {
		t.Fatalf("first InstallLocal: unexpected error: %v", err)
	}

	// Second install should succeed (idempotent)
	if err := InstallLocal(gitDir); err != nil {
		t.Fatalf("second InstallLocal: unexpected error: %v", err)
	}
}

func TestInstallLocalConflict(t *testing.T) {
	gitDir := t.TempDir()
	hooksDir := filepath.Join(gitDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("create hooks dir: %v", err)
	}

	// Write a foreign hook (not written by offstage)
	foreignContent := "#!/bin/sh\necho 'my custom hook'\n"
	p := filepath.Join(hooksDir, "post-checkout")
	if err := os.WriteFile(p, []byte(foreignContent), 0o755); err != nil {
		t.Fatalf("write foreign hook: %v", err)
	}

	err := InstallLocal(gitDir)
	if err == nil {
		t.Fatal("InstallLocal: expected error when foreign hook exists, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("InstallLocal: error %q does not mention 'already exists'", err.Error())
	}
	if !strings.Contains(err.Error(), "not written by offstage") {
		t.Errorf("InstallLocal: error %q does not mention 'not written by offstage'", err.Error())
	}
}

func TestUninstallLocal(t *testing.T) {
	gitDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(gitDir, "hooks"), 0o755); err != nil {
		t.Fatalf("create hooks dir: %v", err)
	}

	// Install first
	if err := InstallLocal(gitDir); err != nil {
		t.Fatalf("InstallLocal: unexpected error: %v", err)
	}

	// Uninstall
	if err := UninstallLocal(gitDir); err != nil {
		t.Fatalf("UninstallLocal: unexpected error: %v", err)
	}

	// Hook files should be gone
	hooksDir := filepath.Join(gitDir, "hooks")
	for _, hookName := range []string{"post-checkout", "pre-push"} {
		p := filepath.Join(hooksDir, hookName)
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("hook file %q still exists after uninstall", hookName)
		}
	}
}

func TestUninstallLocalNothing(t *testing.T) {
	gitDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(gitDir, "hooks"), 0o755); err != nil {
		t.Fatalf("create hooks dir: %v", err)
	}

	// Uninstall with nothing installed should succeed
	if err := UninstallLocal(gitDir); err != nil {
		t.Fatalf("UninstallLocal (nothing): unexpected error: %v", err)
	}
}

func TestUninstallLocalSkipsOtherHooks(t *testing.T) {
	gitDir := t.TempDir()
	hooksDir := filepath.Join(gitDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("create hooks dir: %v", err)
	}

	// Write a foreign pre-push hook
	foreignContent := "#!/bin/sh\necho 'my custom pre-push'\n"
	prePushPath := filepath.Join(hooksDir, "pre-push")
	if err := os.WriteFile(prePushPath, []byte(foreignContent), 0o755); err != nil {
		t.Fatalf("write foreign pre-push hook: %v", err)
	}

	// Write an offstage post-checkout hook manually
	offstageContent := "#!/bin/sh\noffstage hooks run post-checkout \"$@\"\n"
	postCheckoutPath := filepath.Join(hooksDir, "post-checkout")
	if err := os.WriteFile(postCheckoutPath, []byte(offstageContent), 0o755); err != nil {
		t.Fatalf("write offstage post-checkout hook: %v", err)
	}

	// Uninstall should remove post-checkout but leave pre-push alone
	if err := UninstallLocal(gitDir); err != nil {
		t.Fatalf("UninstallLocal: unexpected error: %v", err)
	}

	// post-checkout should be gone
	if _, err := os.Stat(postCheckoutPath); !os.IsNotExist(err) {
		t.Errorf("post-checkout hook still exists after uninstall")
	}

	// pre-push should remain
	content, err := os.ReadFile(prePushPath)
	if err != nil {
		t.Fatalf("pre-push hook was unexpectedly removed: %v", err)
	}
	if string(content) != foreignContent {
		t.Errorf("pre-push hook content changed: got %q, want %q", content, foreignContent)
	}
}

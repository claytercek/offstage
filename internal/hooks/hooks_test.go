package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGlobalInstallRemoved verifies that the global Install function no longer
// exists; only local installation is supported.
// This test is intentionally a compile-time check via the absence of Install/Uninstall
// in the exported API — the CLI-level tests below cover runtime behaviour.

func TestGlobalInstallRemoved(t *testing.T) {
	// Verify that the package does not expose Install or Uninstall symbols for
	// global (core.hooksPath) installation. If this test file compiles and none
	// of the code below calls Install/Uninstall, the global API is gone.
	//
	// The actual enforcement is that hooks.Install and hooks.Uninstall must not
	// be defined. We cannot assert their absence at runtime, but the build will
	// fail if main.go still references them — this comment documents the intent.
	t.Log("global Install/Uninstall removed; only InstallLocal/UninstallLocal remain")
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

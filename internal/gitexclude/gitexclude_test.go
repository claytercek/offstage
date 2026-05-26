package gitexclude

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReplaceManagedBlockAddsPatterns(t *testing.T) {
	got, err := replaceManagedBlock("# local excludes\n", []string{"*.md", ".notes/**"})
	if err != nil {
		t.Fatalf("replaceManagedBlock: %v", err)
	}

	want := strings.Join([]string{
		"# local excludes",
		"",
		beginMarker,
		"*.md",
		".notes/**",
		endMarker,
		"",
	}, "\n")
	if got != want {
		t.Fatalf("managed block mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestReplaceManagedBlockReplacesOnlyManagedBlock(t *testing.T) {
	input := strings.Join([]string{
		"# before",
		beginMarker,
		"old/**",
		endMarker,
		"# after",
		"",
	}, "\n")

	got, err := replaceManagedBlock(input, []string{"new/**"})
	if err != nil {
		t.Fatalf("replaceManagedBlock: %v", err)
	}

	if strings.Contains(got, "old/**") {
		t.Fatalf("stale pattern was not removed:\n%s", got)
	}
	if !strings.Contains(got, "# before") || !strings.Contains(got, "# after") {
		t.Fatalf("user-managed lines were not preserved:\n%s", got)
	}
	if !strings.Contains(got, "new/**") {
		t.Fatalf("new pattern was not written:\n%s", got)
	}
}

func TestReplaceManagedBlockRemovesBlockWhenNoPatterns(t *testing.T) {
	input := strings.Join([]string{
		"# before",
		beginMarker,
		"old/**",
		endMarker,
		"",
	}, "\n")

	got, err := replaceManagedBlock(input, nil)
	if err != nil {
		t.Fatalf("replaceManagedBlock: %v", err)
	}

	want := "# before\n"
	if got != want {
		t.Fatalf("managed block mismatch: got %q, want %q", got, want)
	}
}

func TestReplaceManagedBlockRejectsMalformedBlock(t *testing.T) {
	_, err := replaceManagedBlock(beginMarker+"\nold/**\n", []string{"new/**"})
	if err == nil {
		t.Fatal("expected malformed block error, got nil")
	}
}

func TestSyncWritesGitInfoExclude(t *testing.T) {
	repo := t.TempDir()
	mustGit(t, repo, "init")

	result, err := Sync(repo, []string{"*.md", ".notes/**"})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if !result.Changed {
		t.Fatal("first sync should report changed")
	}

	excludePath := filepath.Join(repo, ".git", "info", "exclude")
	data, err := os.ReadFile(excludePath)
	if err != nil {
		t.Fatalf("read exclude: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "*.md") || !strings.Contains(content, ".notes/**") {
		t.Fatalf("patterns were not written:\n%s", content)
	}

	result, err = Sync(repo, []string{"*.md", ".notes/**"})
	if err != nil {
		t.Fatalf("second Sync: %v", err)
	}
	if result.Changed {
		t.Fatal("second sync should be idempotent")
	}
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

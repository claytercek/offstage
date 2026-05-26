package syncer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/claytercek/offstage/internal/syncer"
)

func TestFileSet_Matches(t *testing.T) {
	var fs syncer.FileSet

	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		// Exact file patterns (used by push include lists).
		{"CONTEXT.md", "CONTEXT.md", true},
		{"CONTEXT.md", "other.md", false},
		{"AGENTS.md", "AGENTS.md", true},

		// Recursive glob patterns (used by push include lists and pull stores).
		{".agents/**", ".agents/rules.md", true},
		{".agents/**", ".agents/sub/deep.md", true},
		{".agents/**", "other/rules.md", false},
		{"docs/adr/**", "docs/adr/0001-foo.md", true},
		{"docs/adr/**", "docs/adr/sub/bar.md", true},
		{"docs/adr/**", "docs/other.md", false},

		// Standard single-star patterns (used by pull glob filtering).
		{"*.md", "README.md", true},
		{"*.md", "subdir/README.md", false},
		{"subdir/*.md", "subdir/README.md", true},
		{"subdir/*.md", "subdir/nested/README.md", false},

		// Exclusion patterns (used by push exclude lists).
		{".agents/secret.md", ".agents/secret.md", true},
		{".agents/secret.md", ".agents/other.md", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.path, func(t *testing.T) {
			got := fs.Matches(tt.pattern, tt.path)
			if got != tt.want {
				t.Errorf("Matches(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}

func TestFileSet_Collect(t *testing.T) {
	var fs syncer.FileSet

	dir := t.TempDir()
	writeFile(t, dir, "CONTEXT.md", "context")
	writeFile(t, dir, "AGENTS.md", "agents")
	writeFile(t, dir, ".agents/rules.md", "rules")
	writeFile(t, dir, ".agents/sub/deep.md", "deep")
	writeFile(t, dir, "docs/adr/0001-foo.md", "adr")
	writeFile(t, dir, "untracked.txt", "ignored")

	t.Run("include patterns from push", func(t *testing.T) {
		include := []string{"CONTEXT.md", "AGENTS.md", ".agents/**", "docs/adr/**"}
		files, err := fs.Collect(dir, include, nil)
		if err != nil {
			t.Fatalf("Collect: %v", err)
		}

		want := map[string]bool{
			"CONTEXT.md":           true,
			"AGENTS.md":            true,
			".agents/rules.md":     true,
			".agents/sub/deep.md":  true,
			"docs/adr/0001-foo.md": true,
		}
		for _, f := range files {
			if !want[f] {
				t.Errorf("unexpected file %q in result", f)
			}
			delete(want, f)
		}
		for f := range want {
			t.Errorf("expected file %q not found in result", f)
		}
	})

	t.Run("exclude patterns used by push", func(t *testing.T) {
		include := []string{"CONTEXT.md", ".agents/**"}
		exclude := []string{".agents/sub/**"}

		files, err := fs.Collect(dir, include, exclude)
		if err != nil {
			t.Fatalf("Collect: %v", err)
		}

		for _, f := range files {
			if f == ".agents/sub/deep.md" {
				t.Error("excluded file .agents/sub/deep.md was included")
			}
		}

		found := false
		for _, f := range files {
			if f == ".agents/rules.md" {
				found = true
			}
		}
		if !found {
			t.Error(".agents/rules.md should be included")
		}
	})

	t.Run("empty include matches nothing", func(t *testing.T) {
		files, err := fs.Collect(dir, []string{}, nil)
		if err != nil {
			t.Fatalf("Collect: %v", err)
		}
		if len(files) != 0 {
			t.Errorf("expected no files with empty include, got %v", files)
		}
	})
}

func TestFileSet_Copy(t *testing.T) {
	var fs syncer.FileSet

	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "test.md")
	if err := os.WriteFile(srcFile, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	dstFile := filepath.Join(dstDir, "test.md")
	if err := fs.Copy(srcFile, dstFile); err != nil {
		t.Fatalf("Copy: %v", err)
	}

	data, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("Copy content mismatch: got %q, want %q", string(data), "hello world")
	}
}

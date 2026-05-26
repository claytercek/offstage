package resolver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/claytercek/offstage/internal/resolver"
)

func TestResolveProjectIDHTTPS(t *testing.T) {
	dir := initRepo(t, "https://github.com/clay/my-project.git")
	got, err := resolver.ResolveProjectID(dir)
	if err != nil {
		t.Fatalf("ResolveProjectID: %v", err)
	}
	want := "github.com/clay/my-project"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveProjectIDSSH(t *testing.T) {
	dir := initRepo(t, "git@github.com:clay/my-project.git")
	got, err := resolver.ResolveProjectID(dir)
	if err != nil {
		t.Fatalf("ResolveProjectID: %v", err)
	}
	want := "github.com/clay/my-project"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveProjectIDGitSuffixStripped(t *testing.T) {
	dirWith := initRepo(t, "https://github.com/clay/my-project.git")
	dirWithout := initRepo(t, "https://github.com/clay/my-project")

	r1, err := resolver.ResolveProjectID(dirWith)
	if err != nil {
		t.Fatalf("ResolveProjectID (with .git): %v", err)
	}
	r2, err := resolver.ResolveProjectID(dirWithout)
	if err != nil {
		t.Fatalf("ResolveProjectID (without .git): %v", err)
	}
	if r1 != r2 {
		t.Errorf("mismatch: %q vs %q", r1, r2)
	}
}

func TestResolveProjectIDLowercased(t *testing.T) {
	dir := initRepo(t, "https://GitHub.COM/Clay/My-Project.git")
	got, err := resolver.ResolveProjectID(dir)
	if err != nil {
		t.Fatalf("ResolveProjectID: %v", err)
	}
	if strings.ToLower(got) != got {
		t.Errorf("not lowercased: %q", got)
	}
	want := "github.com/clay/my-project"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveProjectIDNameOverride(t *testing.T) {
	dir := initRepo(t, "https://github.com/clay/my-project.git")

	rcPath := filepath.Join(dir, ".offstagerc.toml")
	if err := os.WriteFile(rcPath, []byte(`name = "my-custom-project-id"`+"\n"), 0o644); err != nil {
		t.Fatalf("write .offstagerc.toml: %v", err)
	}

	got, err := resolver.ResolveProjectID(dir)
	if err != nil {
		t.Fatalf("ResolveProjectID: %v", err)
	}
	want := "my-custom-project-id"
	if got != want {
		t.Errorf("got %q, want %q (name override should take precedence)", got, want)
	}
}

func TestResolveProjectIDNoRemoteError(t *testing.T) {
	dir := initRepo(t, "")
	_, err := resolver.ResolveProjectID(dir)
	if err == nil {
		t.Fatal("expected error for repo with no remote, got nil")
	}
	if !strings.Contains(err.Error(), "origin") {
		t.Errorf("error should mention 'origin', got: %v", err)
	}
}

func TestResolveProjectIDNotInGitRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := resolver.ResolveProjectID(dir)
	if err == nil {
		t.Fatal("expected error for non-git directory, got nil")
	}
	if !strings.Contains(err.Error(), "git repository") {
		t.Errorf("error should mention 'git repository', got: %v", err)
	}
}

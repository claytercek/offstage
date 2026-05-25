package registry

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/gastownhall/offstage/internal/store"
)

// newStore creates a test store backed by a bare remote with an initial commit
// on the main branch.
func newStore(t *testing.T) *store.Store {
	t.Helper()

	remote := t.TempDir()
	if out, err := exec.Command("git", "init", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}

	local := t.TempDir()
	if out, err := exec.Command("git", "clone", remote, local).CombinedOutput(); err != nil {
		t.Fatalf("git clone: %v\n%s", err, out)
	}

	// Configure identity for commits.
	exec.Command("git", "-C", local, "config", "user.email", "test@test.com").Run()  //nolint
	exec.Command("git", "-C", local, "config", "user.name", "Test").Run()            //nolint

	// Create an initial commit on main so the branch exists.
	if out, err := exec.Command("git", "-C", local, "commit", "--allow-empty", "-m", "init").CombinedOutput(); err != nil {
		t.Fatalf("initial commit: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", local, "push", "origin", "HEAD").CombinedOutput(); err != nil {
		t.Fatalf("initial push: %v\n%s", err, out)
	}

	s, err := store.Open(local)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	return s
}

func TestUpdateStore_CreatesManifest(t *testing.T) {
	s := newStore(t)

	if err := UpdateStore(s, "github.com/clay/my-project", "feature-x"); err != nil {
		t.Fatalf("UpdateStore: %v", err)
	}

	// Verify manifest.toml exists on main branch via git show.
	out, err := s.ExecOutput("show", "main:manifest.toml")
	if err != nil {
		t.Fatalf("manifest.toml not found on main: %v", err)
	}
	if !strings.Contains(out, "feature-x") {
		t.Errorf("manifest.toml does not contain 'feature-x':\n%s", out)
	}
	if !strings.Contains(out, "github.com/clay/my-project") {
		t.Errorf("manifest.toml does not contain project ID:\n%s", out)
	}
}

func TestCheckUnreconciled_NoMainBranch(t *testing.T) {
	// Create a store where the main branch doesn't exist at all.
	remote := t.TempDir()
	exec.Command("git", "init", "--bare", remote).Run() //nolint

	local := t.TempDir()
	exec.Command("git", "clone", remote, local).Run()                                //nolint
	exec.Command("git", "-C", local, "config", "user.email", "test@test.com").Run() //nolint
	exec.Command("git", "-C", local, "config", "user.name", "Test").Run()           //nolint

	// Create an initial commit on a non-main branch.
	exec.Command("git", "-C", local, "checkout", "-b", "other").Run()                                                          //nolint
	exec.Command("git", "-C", local, "commit", "--allow-empty", "-m", "other init").Run() //nolint

	s, err := store.Open(local)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	result, err := CheckUnreconciled(s, "github.com/clay/my-project", "main")
	if err != nil {
		t.Fatalf("CheckUnreconciled returned unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result when main branch doesn't exist, got: %v", result)
	}
}

func TestCheckUnreconciled_WithUnreconciled(t *testing.T) {
	s := newStore(t)

	// Seed two unreconciled branches and one reconciled via UpdateStore.
	if err := UpdateStore(s, "github.com/clay/my-project", "feature-x"); err != nil {
		t.Fatalf("UpdateStore feature-x: %v", err)
	}
	if err := UpdateStore(s, "github.com/clay/my-project", "feature-y"); err != nil {
		t.Fatalf("UpdateStore feature-y: %v", err)
	}
	if err := UpdateStore(s, "github.com/clay/my-project", "main"); err != nil {
		t.Fatalf("UpdateStore main: %v", err)
	}

	// Reconcile "main" by directly editing the manifest on the main branch.
	// We need to check out main, edit, commit, push.
	if err := s.Checkout("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	m, err := Load(s.Path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	MarkReconciled(m, "github.com/clay/my-project", "main")
	if err := Save(s.Path, m); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := s.Add(manifestFile); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := s.Commit("registry: mark main reconciled"); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := s.Push(); err != nil {
		t.Fatalf("Push: %v", err)
	}

	// Now CheckUnreconciled from "main" should return feature-x and feature-y.
	unreconciled, err := CheckUnreconciled(s, "github.com/clay/my-project", "main")
	if err != nil {
		t.Fatalf("CheckUnreconciled: %v", err)
	}
	if len(unreconciled) != 2 {
		t.Fatalf("expected 2 unreconciled branches, got %d: %v", len(unreconciled), unreconciled)
	}
	has := func(name string) bool {
		for _, b := range unreconciled {
			if b == name {
				return true
			}
		}
		return false
	}
	if !has("feature-x") {
		t.Error("expected feature-x in unreconciled list")
	}
	if !has("feature-y") {
		t.Error("expected feature-y in unreconciled list")
	}
	// Sanity: reconciled "main" branch must NOT appear.
	if has("main") {
		t.Error("reconciled 'main' should not appear in unreconciled list")
	}
}

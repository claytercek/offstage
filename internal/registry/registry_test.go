package registry

import (
	"testing"
)

func TestRegister(t *testing.T) {
	m := &Manifest{}
	modified := Register(m, "github.com/clay/my-project", "feature-x")
	if !modified {
		t.Fatal("expected modified=true on first Register")
	}
	if len(m.Branches) != 1 {
		t.Fatalf("expected 1 branch, got %d", len(m.Branches))
	}
	rec := m.Branches[0]
	if rec.Project != "github.com/clay/my-project" {
		t.Errorf("wrong project: %s", rec.Project)
	}
	if rec.Branch != "feature-x" {
		t.Errorf("wrong branch: %s", rec.Branch)
	}
	if rec.Reconciled {
		t.Error("expected reconciled=false for a freshly registered branch")
	}
}

func TestRegisterIdempotent(t *testing.T) {
	m := &Manifest{}
	Register(m, "github.com/clay/my-project", "feature-x")

	// Second registration must be a no-op.
	modified := Register(m, "github.com/clay/my-project", "feature-x")
	if modified {
		t.Fatal("expected modified=false when re-registering the same branch")
	}
	if len(m.Branches) != 1 {
		t.Fatalf("expected exactly 1 branch after idempotent Register, got %d", len(m.Branches))
	}
}

func TestMarkReconciled(t *testing.T) {
	m := &Manifest{}
	Register(m, "github.com/clay/my-project", "feature-x")

	modified := MarkReconciled(m, "github.com/clay/my-project", "feature-x")
	if !modified {
		t.Fatal("expected modified=true on first MarkReconciled")
	}
	if !m.Branches[0].Reconciled {
		t.Error("expected reconciled=true after MarkReconciled")
	}

	// Marking again should be a no-op.
	modified = MarkReconciled(m, "github.com/clay/my-project", "feature-x")
	if modified {
		t.Fatal("expected modified=false when already reconciled")
	}
}

func TestUnreconciledBranches(t *testing.T) {
	m := &Manifest{}
	Register(m, "github.com/clay/my-project", "main")
	Register(m, "github.com/clay/my-project", "feature-x")
	Register(m, "github.com/clay/my-project", "feature-y")

	// Reconcile only "main".
	MarkReconciled(m, "github.com/clay/my-project", "main")

	// When current branch is "main", the two unreconciled ones should be returned.
	unreconciled := UnreconciledBranches(m, "github.com/clay/my-project", "main")
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

	// The current branch itself should never appear.
	for _, b := range unreconciled {
		if b == "main" {
			t.Error("current branch 'main' should not appear in unreconciled list")
		}
	}
}

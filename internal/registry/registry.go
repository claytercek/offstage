// Package registry maintains the store-level manifest that records which
// project branches exist in the sync store and whether they have been
// reconciled after a PR lands.
//
// The manifest is stored as manifest.toml on the "main" branch of the sync
// store. This is distinct from the per-project .offstagerc.toml managed by
// the internal/manifest package.
package registry

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const manifestFile = "manifest.toml"

// BranchRecord represents a single project+branch entry in the store manifest.
type BranchRecord struct {
	Project    string `toml:"project"`
	Branch     string `toml:"branch"`
	Reconciled bool   `toml:"reconciled"`
	MergedInto string `toml:"merged_into,omitempty"`
}

// Manifest is the top-level structure of the store-level manifest.toml file.
type Manifest struct {
	Branches []BranchRecord `toml:"branches"`
}

// Load reads manifest.toml from storeDir.
// Returns an empty manifest (no error) if the file does not exist.
func Load(storeDir string) (*Manifest, error) {
	p := filepath.Join(storeDir, manifestFile)
	f, err := os.Open(p)
	if errors.Is(err, os.ErrNotExist) {
		return &Manifest{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", manifestFile, err)
	}
	defer f.Close()

	var m Manifest
	if _, err := toml.NewDecoder(f).Decode(&m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", manifestFile, err)
	}
	return &m, nil
}

// Save writes manifest.toml to storeDir.
func Save(storeDir string, m *Manifest) error {
	p := filepath.Join(storeDir, manifestFile)
	f, err := os.Create(p)
	if err != nil {
		return fmt.Errorf("create %s: %w", manifestFile, err)
	}
	defer f.Close()
	if err := toml.NewEncoder(f).Encode(m); err != nil {
		return fmt.Errorf("write %s: %w", manifestFile, err)
	}
	return nil
}

// Register ensures that projectID/branchName is recorded in the manifest.
// Returns true if the manifest was modified (a new entry was added).
// Returns false if the entry already existed.
func Register(m *Manifest, projectID, branchName string) bool {
	for _, rec := range m.Branches {
		if rec.Project == projectID && rec.Branch == branchName {
			return false
		}
	}
	m.Branches = append(m.Branches, BranchRecord{
		Project:    projectID,
		Branch:     branchName,
		Reconciled: false,
	})
	return true
}

// MarkReconciled marks the projectID/branchName entry as reconciled.
// Returns true if the manifest was modified (the flag was changed from false to
// true, or the entry was created). Returns false if the entry was already
// reconciled.
func MarkReconciled(m *Manifest, projectID, branchName string) bool {
	for i, rec := range m.Branches {
		if rec.Project == projectID && rec.Branch == branchName {
			if rec.Reconciled {
				return false
			}
			m.Branches[i].Reconciled = true
			return true
		}
	}
	// Entry doesn't exist yet — create it as reconciled.
	m.Branches = append(m.Branches, BranchRecord{
		Project:    projectID,
		Branch:     branchName,
		Reconciled: true,
	})
	return true
}

// UnreconciledBranches returns the branch names of branches belonging to
// projectID that are not reconciled and are not the currentBranch itself.
func UnreconciledBranches(m *Manifest, projectID, currentBranch string) []string {
	var result []string
	for _, rec := range m.Branches {
		if rec.Project == projectID && !rec.Reconciled && rec.Branch != currentBranch {
			result = append(result, rec.Branch)
		}
	}
	return result
}

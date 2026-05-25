package registry

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/gastownhall/offstage/internal/store"
)

// UpdateStore updates manifest.toml on the store's main branch.
//
// Flow:
//  1. Save the current branch name in the store.
//  2. Checkout (or create) the main branch in the store.
//  3. Load (or create) manifest.toml.
//  4. Call Register for the given projectID+branchName.
//  5. If modified: Save manifest, stage, commit, push.
//  6. Checkout back to the original branch.
//
// This is called after a successful push.
func UpdateStore(s *store.Store, projectID, branchName string) error {
	// 1. Remember the current branch so we can restore it afterwards.
	original, err := s.CurrentBranch()
	if err != nil {
		return fmt.Errorf("get current branch: %w", err)
	}

	// 2. Switch to (or create) the main branch.
	if s.BranchExists("main") {
		if err := s.Checkout("main"); err != nil {
			return fmt.Errorf("checkout main: %w", err)
		}
	} else {
		if err := s.CreateBranch("main"); err != nil {
			return fmt.Errorf("create main branch: %w", err)
		}
	}

	// Restore the original branch on exit regardless of outcome.
	defer func() {
		_ = s.Checkout(original)
	}()

	// 3. Load or create manifest.
	m, err := Load(s.Path)
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}

	// 4. Register the branch.
	modified := Register(m, projectID, branchName)
	if !modified {
		// Nothing changed; no commit needed.
		return nil
	}

	// 5. Save, stage, commit, push.
	if err := Save(s.Path, m); err != nil {
		return fmt.Errorf("save manifest: %w", err)
	}
	if err := s.Add(manifestFile); err != nil {
		return fmt.Errorf("stage manifest: %w", err)
	}
	msg := fmt.Sprintf("registry: register %s/%s", projectID, branchName)
	if err := s.Commit(msg); err != nil {
		return fmt.Errorf("commit manifest: %w", err)
	}
	if err := s.Push(); err != nil {
		return fmt.Errorf("push manifest: %w", err)
	}

	return nil
}

// CheckUnreconciled fetches manifest.toml from the store's main branch and
// returns the names of unreconciled branches for projectID that are not
// currentBranch.
//
// It reads via "git show" so it does NOT change the store's checked-out branch.
// Returns nil (no warnings) if the main branch or manifest.toml does not exist.
func CheckUnreconciled(s *store.Store, projectID, currentBranch string) ([]string, error) {
	// Fetch so we see the latest main.
	// Best-effort: if fetch fails we work with whatever is cached locally.
	_ = s.Fetch()

	out, err := s.ExecOutput("show", "main:manifest.toml")
	if err != nil {
		// main branch doesn't exist or manifest.toml isn't there — no warnings.
		return nil, nil
	}

	var m Manifest
	if _, err := toml.Decode(out, &m); err != nil {
		// Corrupt manifest — log and return no warnings rather than crashing.
		return nil, fmt.Errorf("parse store manifest: %w", err)
	}

	branches := UnreconciledBranches(&m, projectID, currentBranch)
	// Filter out any empty strings that might sneak in.
	var result []string
	for _, b := range branches {
		if strings.TrimSpace(b) != "" {
			result = append(result, b)
		}
	}
	return result, nil
}

// Package syncenv provides the SyncEnv type, which consolidates the four
// initialization steps common to most offstage commands: loading the global
// config, loading the per-project manifest, resolving the project identity,
// and opening the sync store.
package syncenv

import (
	"fmt"

	"github.com/claytercek/offstage/internal/config"
	"github.com/claytercek/offstage/internal/manifest"
	"github.com/claytercek/offstage/internal/resolver"
	"github.com/claytercek/offstage/internal/store"
)

// SyncEnv holds the fully-initialized context needed by sync commands.
type SyncEnv struct {
	Config   *config.Config
	Manifest *manifest.ProjectConfig
	Resolved *resolver.Result
	Store    *store.Store
}

// Open loads the global config, the per-project manifest rooted at the repo,
// resolves the current project identity, and opens the sync store. All four
// steps must succeed; the first failure returns an error.
func Open(cwd string) (*SyncEnv, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	res, err := resolver.Resolve(cwd)
	if err != nil {
		return nil, fmt.Errorf("resolve project: %w", err)
	}

	mf, err := manifest.Load(res.RepoRoot)
	if err != nil {
		return nil, fmt.Errorf("load manifest: %w", err)
	}

	s, err := store.Open(cfg.StorePath)
	if err != nil {
		return nil, err
	}

	return &SyncEnv{
		Config:   cfg,
		Manifest: mf,
		Resolved: res,
		Store:    s,
	}, nil
}

// OpenWithoutManifest loads the global config, resolves the current project
// identity, and opens the sync store, but does not load the per-project
// manifest. Use this for commands that operate on the store by project/branch
// identity but do not need the file include/exclude lists.
func OpenWithoutManifest(cwd string) (*SyncEnv, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	res, err := resolver.Resolve(cwd)
	if err != nil {
		return nil, fmt.Errorf("resolve project: %w", err)
	}

	s, err := store.Open(cfg.StorePath)
	if err != nil {
		return nil, err
	}

	return &SyncEnv{
		Config:   cfg,
		Manifest: nil,
		Resolved: res,
		Store:    s,
	}, nil
}

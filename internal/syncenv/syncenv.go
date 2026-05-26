// Package syncenv provides the SyncEnv type, which consolidates the
// initialization steps common to most offstage commands: loading the global
// config, resolving the project identity, opening the sync store, and
// optionally loading the per-project manifest.
//
// Commands declare what they need by passing Option values to Open rather than
// choosing between fixed entry-point variants.
package syncenv

import (
	"fmt"

	"github.com/claytercek/offstage/internal/config"
	"github.com/claytercek/offstage/internal/manifest"
	"github.com/claytercek/offstage/internal/resolver"
	"github.com/claytercek/offstage/internal/store"
)

// SyncEnv holds the fully-initialized context needed by sync commands.
// Fields that were not requested via options are nil.
type SyncEnv struct {
	Config   *config.Config
	Manifest *manifest.ProjectConfig
	Resolved *resolver.Result
	Store    *store.Store
}

// options holds the set of initialization steps requested by the caller.
type options struct {
	manifest bool
}

// Option configures which initialization steps Open performs.
type Option func(*options)

// WithManifest instructs Open to load the per-project manifest
// (.offstagerc.toml) from the repository root. When not supplied, SyncEnv.Manifest
// is nil.
func WithManifest() Option {
	return func(o *options) {
		o.manifest = true
	}
}

// Open loads the global config, resolves the current project identity, and
// opens the sync store. Callers may supply additional Option values to declare
// which optional initialization steps are also required. The first failure
// returns an error.
//
// Example — load everything including the manifest:
//
//	env, err := syncenv.Open(cwd, syncenv.WithManifest())
//
// Example — skip the manifest (store-only commands):
//
//	env, err := syncenv.Open(cwd)
func Open(cwd string, opts ...Option) (*SyncEnv, error) {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

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

	env := &SyncEnv{
		Config:   cfg,
		Resolved: res,
		Store:    s,
	}

	if o.manifest {
		mf, err := manifest.Load(res.RepoRoot)
		if err != nil {
			return nil, fmt.Errorf("load manifest: %w", err)
		}
		env.Manifest = mf
	}

	return env, nil
}

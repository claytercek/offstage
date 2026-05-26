package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/claytercek/offstage/internal/config"
	"github.com/claytercek/offstage/internal/store"
)

var initCmd = &cobra.Command{
	Use:   "init <git-url>",
	Short: "Initialize offstage by cloning the sync store",
	Long: `Clone the sync store repository from the given git URL and write the
global config at ~/.config/offstage/config.toml.

Run 'offstage init' only once per machine. If the store has already been
initialized, offstage prints an error rather than clobbering your data.`,
	Args: cobra.ExactArgs(1),
	RunE: runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	gitURL := args[0]

	// Detect an already-initialized config so we can give a clear error.
	existing, err := config.Load()
	if err != nil && !errors.Is(err, config.ErrNotInitialized) {
		return fmt.Errorf("check existing config: %w", err)
	}
	if existing != nil {
		return fmt.Errorf("offstage is already initialized (store: %s); remove %s to re-initialize",
			existing.StorePath, mustConfigPath())
	}

	storePath, err := config.DefaultStorePath()
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Cloning sync store from %s → %s\n", gitURL, storePath)

	if err := store.Clone(gitURL, storePath); err != nil {
		return err
	}

	cfg := &config.Config{
		StoreURL:  gitURL,
		StorePath: storePath,
	}
	if err := config.Write(cfg); err != nil {
		return err
	}

	cfgPath, _ := config.Path()
	fmt.Fprintf(cmd.OutOrStdout(), "offstage initialized.\n  store:  %s\n  config: %s\n", storePath, cfgPath)
	return nil
}

func mustConfigPath() string {
	p, err := config.Path()
	if err != nil {
		return "~/.config/offstage/config.toml"
	}
	return p
}

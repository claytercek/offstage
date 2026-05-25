package main

import (
	"fmt"
	"os"

	"github.com/gastownhall/offstage/internal/manifest"
	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		// cobra already prints the error; just exit non-zero.
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "offstage",
	Short: "Sync gitignored personal files across machines",
	Long: `offstage syncs your gitignored personal files — CONTEXT.md, ADRs, AGENTS.md,
and similar artefacts — across machines using a private git repository as the
sync store.

Quick start:
  offstage init <git-url>   Initialize with your private sync store
  offstage track <pattern>  Track a file or glob
  offstage push             Push local changes to the store
  offstage pull             Pull changes from the store
`,
}

func init() {
	// Subcommands registered here so rootCmd is ready before Execute().
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(trackCmd)
	rootCmd.AddCommand(untrackCmd)
	rootCmd.AddCommand(pushCmd)
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(gitCmd)
	rootCmd.AddCommand(mergeCmd)
}

// ---------------------------------------------------------------------------
// Planned subcommands — stubs so that --help lists them.
// Full implementations live in separate issues.
// ---------------------------------------------------------------------------

var trackCmd = &cobra.Command{
	Use:   "track <pattern>",
	Short: "Track a file or glob pattern in the current project",
	Long:  "Add a file or glob pattern to the sync config for this project. (offstage-1v1)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		cfg, err := manifest.Load(cwd)
		if err != nil {
			return fmt.Errorf("load manifest: %w", err)
		}
		pattern := args[0]
		if !manifest.AddPattern(cfg, pattern) {
			fmt.Fprintf(cmd.OutOrStdout(), "already tracking %q\n", pattern)
			return nil
		}
		if err := manifest.Write(cwd, cfg); err != nil {
			return fmt.Errorf("write manifest: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "tracking %q\n", pattern)
		return nil
	},
}

var untrackCmd = &cobra.Command{
	Use:   "untrack <pattern>",
	Short: "Stop tracking a file or glob pattern",
	Long:  "Remove a file or glob pattern from the sync config for this project. (offstage-1v1)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		cfg, err := manifest.Load(cwd)
		if err != nil {
			return fmt.Errorf("load manifest: %w", err)
		}
		pattern := args[0]
		if !manifest.RemovePattern(cfg, pattern) {
			fmt.Fprintf(cmd.OutOrStdout(), "%q is not tracked\n", pattern)
			return nil
		}
		if err := manifest.Write(cwd, cfg); err != nil {
			return fmt.Errorf("write manifest: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "no longer tracking %q\n", pattern)
		return nil
	},
}

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push local changes to the sync store",
	Long:  "Copy tracked files from the local filesystem into the sync store and commit. (offstage-393)",
	RunE:  notImplemented,
}

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull changes from the sync store to the local filesystem",
	Long:  "Fetch and apply tracked files from the sync store to the local filesystem. (offstage-393)",
	RunE:  notImplemented,
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync status for the current project",
	Long:  "Display which tracked files differ between local and the store. (offstage-393)",
	RunE:  notImplemented,
}

var gitCmd = &cobra.Command{
	Use:                "git",
	Short:              "Run a git command inside the sync store",
	Long:               "Pass arbitrary git commands through to the sync store repository. (offstage-393)",
	DisableFlagParsing: true,
	RunE:               notImplemented,
}

var mergeCmd = &cobra.Command{
	Use:   "merge <branch>",
	Short: "Merge a branch context into the current branch",
	Long: `Explicitly reconcile a sync-store branch into the current branch context.
Required after a PR lands if the branch context should flow to the target branch.
See ADR-0001 for rationale. (offstage-qim)`,
	Args: cobra.ExactArgs(1),
	RunE: notImplemented,
}

func notImplemented(cmd *cobra.Command, _ []string) error {
	return fmt.Errorf("%s: not yet implemented", cmd.Use)
}

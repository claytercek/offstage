package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/gastownhall/offstage/internal/config"
	"github.com/gastownhall/offstage/internal/manifest"
	"github.com/gastownhall/offstage/internal/registry"
	"github.com/gastownhall/offstage/internal/resolver"
	"github.com/gastownhall/offstage/internal/store"
	"github.com/gastownhall/offstage/internal/syncer"
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
	rootCmd.AddCommand(diffCmd)

	// push flags
	pushCmd.Flags().Bool("dry-run", false, "Print files that would be pushed without modifying the store")
	pushCmd.Flags().Bool("global", false, "Push globally tracked files instead of project files")
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
	RunE:  runPush,
}

func runPush(cmd *cobra.Command, _ []string) error {
	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		return fmt.Errorf("get dry-run flag: %w", err)
	}
	globalFlag, err := cmd.Flags().GetBool("global")
	if err != nil {
		return fmt.Errorf("get global flag: %w", err)
	}

	// Load global config to get the store path.
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Use global mode if --global flag is set, or if we're not in a git repo.
	useGlobal := globalFlag
	if !useGlobal {
		if _, err := resolver.Resolve(cwd); err != nil {
			useGlobal = true
		}
	}

	if useGlobal {
		s, err := store.Open(cfg.StorePath)
		if err != nil {
			return err
		}
		includes := cfg.Global.Include
		if len(includes) == 0 {
			includes = config.DefaultGlobalPatterns()
		}
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("get home dir: %w", err)
		}
		if cfg.Global.HomeDir != "" {
			homeDir = cfg.Global.HomeDir
		}
		return syncer.PushGlobal(s, homeDir, includes, dryRun)
	}

	// Project push path.
	mf, err := manifest.Load(cwd)
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}

	res, err := resolver.Resolve(cwd)
	if err != nil {
		return fmt.Errorf("resolve project: %w", err)
	}

	s, err := store.Open(cfg.StorePath)
	if err != nil {
		return err
	}

	if err := syncer.Push(s, cwd, mf.Include, mf.Exclude, res.ProjectID, res.BranchName, dryRun); err != nil {
		return err
	}

	// Update branch registry on store's main branch.
	if err := registry.UpdateStore(s, res.ProjectID, res.BranchName); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not update branch registry: %v\n", err)
		// Non-fatal: don't fail the push
	}
	return nil
}

var pullDryRun bool
var pullGlobal bool

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull changes from the sync store to the local filesystem",
	Long:  "Fetch and apply tracked files from the sync store to the local filesystem. (offstage-6qp)",
	RunE:  runPull,
}

func init() {
	pullCmd.Flags().BoolVar(&pullDryRun, "dry-run", false, "Print files that would be pulled without writing them")
	pullCmd.Flags().BoolVar(&pullGlobal, "global", false, "Pull globally tracked files instead of project files")
}

func runPull(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Use global mode if --global flag is set, or if we're not in a git repo.
	useGlobal := pullGlobal
	if !useGlobal {
		if _, err := resolver.Resolve(cwd); err != nil {
			useGlobal = true
		}
	}

	if useGlobal {
		s, err := store.Open(cfg.StorePath)
		if err != nil {
			return err
		}
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("get home dir: %w", err)
		}
		if cfg.Global.HomeDir != "" {
			homeDir = cfg.Global.HomeDir
		}
		if err := syncer.PullGlobal(s, homeDir, pullDryRun); err != nil {
			if errors.Is(err, syncer.ErrBranchNotFound) || errors.Is(err, syncer.ErrDiverged) {
				fmt.Fprintln(cmd.ErrOrStderr(), err)
				os.Exit(1)
			}
			return err
		}
		return nil
	}

	res, err := resolver.Resolve(cwd)
	if err != nil {
		return fmt.Errorf("resolve project: %w", err)
	}

	s, err := store.Open(cfg.StorePath)
	if err != nil {
		return err
	}

	if err := syncer.Pull(s, cwd, res.ProjectID, res.BranchName, pullDryRun); err != nil {
		if errors.Is(err, syncer.ErrBranchNotFound) || errors.Is(err, syncer.ErrDiverged) {
			fmt.Fprintln(cmd.ErrOrStderr(), err)
			os.Exit(1)
		}
		return err
	}

	// Warn about unreconciled branches.
	unreconciled, err := registry.CheckUnreconciled(s, res.ProjectID, res.BranchName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not check branch registry: %v\n", err)
	} else if len(unreconciled) > 0 {
		fmt.Fprintf(os.Stderr, "\n⚠  Unreconciled branches: %s\n", strings.Join(unreconciled, ", "))
		fmt.Fprintf(os.Stderr, "   Run 'offstage merge <branch>' after these PRs land.\n")
	}
	return nil
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
	RunE:               runGit,
}

func runGit(_ *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	s, err := store.Open(cfg.StorePath)
	if err != nil {
		return err
	}
	return s.Exec(args...)
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

var diffCmd = &cobra.Command{
	Use:   "diff [<branch>]",
	Short: "Show diff between local and sync store state",
	Long: `Show unified diff between the local tracked files and the sync store source state.

With no argument, compares each local tracked file against the version stored in
the sync store for the current project branch.

With a branch argument, compares the current project branch in the sync store
against the named branch (useful for reviewing context differences across branches).

Exit code is 0 if no differences, 1 if differences exist (POSIX diff convention).`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDiff,
}

func runDiff(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	res, err := resolver.Resolve(cwd)
	if err != nil {
		return fmt.Errorf("resolve project: %w", err)
	}

	s, err := store.Open(cfg.StorePath)
	if err != nil {
		return err
	}

	if len(args) == 1 {
		currentStoreBranch := res.ProjectID + "/" + res.BranchName
		targetStoreBranch := res.ProjectID + "/" + args[0]
		err = syncer.DiffBranches(s, currentStoreBranch, targetStoreBranch)
	} else {
		mf, loadErr := manifest.Load(cwd)
		if loadErr != nil {
			return fmt.Errorf("load manifest: %w", loadErr)
		}
		storeBranch := res.ProjectID + "/" + res.BranchName
		err = syncer.DiffLocal(s, cwd, mf.Include, mf.Exclude, storeBranch)
	}

	if errors.Is(err, syncer.ErrHasDiff) {
		// git diff already printed the diff; just set exit code 1 silently.
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), err)
		os.Exit(1)
	}
	return nil
}

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/claytercek/offstage/internal/config"
	"github.com/claytercek/offstage/internal/gitexclude"
	"github.com/claytercek/offstage/internal/hooks"
	"github.com/claytercek/offstage/internal/manifest"
	"github.com/claytercek/offstage/internal/resolver"
	"github.com/claytercek/offstage/internal/store"
	"github.com/claytercek/offstage/internal/syncenv"
	"github.com/claytercek/offstage/internal/syncer"
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
	Long: `offstage syncs gitignored personal project files across machines using
a private git repository as the sync store.

Quick start:
  offstage init <git-url>   Initialize with your private sync store
  offstage track <pattern>  Track a file or pattern
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
	rootCmd.AddCommand(gitExcludeCmd)
	rootCmd.AddCommand(mergeCmd)
	rootCmd.AddCommand(diffCmd)
	rootCmd.AddCommand(hooksCmd)
	hooksCmd.AddCommand(hooksInstallCmd)
	hooksCmd.AddCommand(hooksUninstallCmd)
	hooksCmd.AddCommand(hooksRunCmd)

	// hooks install/uninstall only support local (.git/hooks/) mode; no flags needed.

	trackCmd.Flags().BoolVar(&trackSyncExclude, "sync-exclude", false, "Also sync tracked patterns to .git/info/exclude")
	untrackCmd.Flags().BoolVar(&untrackSyncExclude, "sync-exclude", false, "Also sync tracked patterns to .git/info/exclude")

	// push flags
	pushCmd.Flags().Bool("dry-run", false, "Print files that would be pushed without modifying the store")
}

// ---------------------------------------------------------------------------
// Planned subcommands — stubs so that --help lists them.
// Full implementations live in separate issues.
// ---------------------------------------------------------------------------

var trackSyncExclude bool

var trackCmd = &cobra.Command{
	Use:   "track <pattern>",
	Short: "Track a file or Git ignore pattern in the current project",
	Long:  "Add a file or Git ignore pattern to the project manifest. (offstage-1v1)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		repoRoot, err := resolver.RepositoryRoot(cwd)
		if err != nil {
			return fmt.Errorf("resolve repository root: %w", err)
		}
		cfg, err := manifest.Load(repoRoot)
		if err != nil {
			return fmt.Errorf("load manifest: %w", err)
		}
		pattern := args[0]
		if !manifest.AddPattern(cfg, pattern) {
			fmt.Fprintf(cmd.OutOrStdout(), "already tracking %q\n", pattern)
		} else {
			if err := manifest.Write(repoRoot, cfg); err != nil {
				return fmt.Errorf("write manifest: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "tracking %q\n", pattern)
		}
		return syncGitExcludeIfRequested(cmd, repoRoot, cfg, trackSyncExclude)
	},
}

var untrackSyncExclude bool

var untrackCmd = &cobra.Command{
	Use:   "untrack <pattern>",
	Short: "Stop tracking a file or Git ignore pattern",
	Long:  "Remove a file or Git ignore pattern from the project manifest. (offstage-1v1)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		repoRoot, err := resolver.RepositoryRoot(cwd)
		if err != nil {
			return fmt.Errorf("resolve repository root: %w", err)
		}
		cfg, err := manifest.Load(repoRoot)
		if err != nil {
			return fmt.Errorf("load manifest: %w", err)
		}
		pattern := args[0]
		if !manifest.RemovePattern(cfg, pattern) {
			fmt.Fprintf(cmd.OutOrStdout(), "%q is not tracked\n", pattern)
		} else {
			if err := manifest.Write(repoRoot, cfg); err != nil {
				return fmt.Errorf("write manifest: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "no longer tracking %q\n", pattern)
		}
		return syncGitExcludeIfRequested(cmd, repoRoot, cfg, untrackSyncExclude)
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

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	env, err := syncenv.Open(cwd, syncenv.WithManifest())
	if err != nil {
		return err
	}

	if !dryRun {
		if err := syncGitExcludeIfEnabled(env.Resolved.RepoRoot, env.Manifest); err != nil {
			return err
		}
	}

	if err := syncer.Push(env.Store, env.Resolved.RepoRoot, manifest.EffectiveInclude(env.Manifest), env.Manifest.Exclude, env.Resolved.ProjectID, env.Resolved.BranchName, dryRun); err != nil {
		return err
	}
	return nil
}

var pullDryRun bool

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull changes from the sync store to the local filesystem",
	Long:  "Fetch and apply tracked files from the sync store to the local filesystem. (offstage-6qp)",
	RunE:  runPull,
}

func init() {
	pullCmd.Flags().BoolVar(&pullDryRun, "dry-run", false, "Print files that would be pulled without writing them")
}

func runPull(cmd *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	env, err := syncenv.Open(cwd)
	if err != nil {
		return err
	}

	if err := syncer.Pull(env.Store, env.Resolved.RepoRoot, env.Resolved.ProjectID, env.Resolved.BranchName, pullDryRun); err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), err)
		os.Exit(syncer.ExitCode(err))
	}
	return nil
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync status for the current project",
	Long:  "Display which tracked files differ between local and the store. (offstage-393)",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	env, err := syncenv.Open(cwd, syncenv.WithManifest())
	if err != nil {
		return err
	}
	if err := syncGitExcludeIfEnabled(env.Resolved.RepoRoot, env.Manifest); err != nil {
		return err
	}

	storeBranch := env.Resolved.ProjectID + "/" + env.Resolved.BranchName
	if err := syncer.Status(env.Store, env.Resolved.RepoRoot, manifest.EffectiveInclude(env.Manifest), env.Manifest.Exclude, storeBranch); err != nil {
		if !errors.Is(err, syncer.ErrHasDiff) {
			fmt.Fprintln(cmd.ErrOrStderr(), err)
		}
		os.Exit(syncer.ExitCode(err))
	}
	return nil
}

var gitCmd = &cobra.Command{
	Use:                "git",
	Short:              "Run a git command inside the sync store",
	Long:               "Pass arbitrary git commands through to the sync store repository. (offstage-393)",
	DisableFlagParsing: true,
	RunE:               runGit,
}

var gitExcludeCmd = &cobra.Command{
	Use:   "git-exclude",
	Short: "Sync tracked patterns to .git/info/exclude",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		repoRoot, err := resolver.RepositoryRoot(cwd)
		if err != nil {
			return fmt.Errorf("resolve repository root: %w", err)
		}
		cfg, err := manifest.Load(repoRoot)
		if err != nil {
			return fmt.Errorf("load manifest: %w", err)
		}
		result, err := gitexclude.Sync(repoRoot, manifest.EffectiveInclude(cfg))
		if err != nil {
			return err
		}
		printGitExcludeResult(cmd, result)
		return nil
	},
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
	fullArgs := append([]string{"-C", s.Path}, args...)
	cmd := exec.Command("git", fullArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

var mergeCmd = &cobra.Command{
	Use:   "merge <branch>",
	Short: "Merge a branch context into the current branch",
	Long: `Explicitly reconcile a sync-store branch into the current branch context.
Required after a PR lands if the branch context should flow to the target branch.
See ADR-0001 for rationale. (offstage-qim)`,
	Args: cobra.ExactArgs(1),
	RunE: runMerge,
}

func runMerge(_ *cobra.Command, args []string) error {
	sourceBranch := args[0]

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	env, err := syncenv.Open(cwd)
	if err != nil {
		return err
	}

	if err := syncer.Merge(env.Store, env.Resolved.ProjectID, env.Resolved.BranchName, sourceBranch); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(syncer.ExitCode(err))
	}

	fmt.Printf("Merged %s into %s and pushed.\n", sourceBranch, env.Resolved.BranchName)
	return nil
}

var hooksCmd = &cobra.Command{
	Use:   "hooks",
	Short: "Manage git hooks for automated offstage sync",
}

var hooksInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install offstage git hooks into the current repo's .git/hooks/",
	Long: `Install offstage post-checkout and pre-push hooks into the current
repository's .git/hooks/ directory.

Only local installation (into .git/hooks/) is supported. This avoids touching
the global git config and silently overriding hooks in unrelated repositories.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		gd, err := getGitDir()
		if err != nil {
			return err
		}
		return hooks.InstallLocal(gd)
	},
}

var hooksUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall offstage git hooks from the current repo's .git/hooks/",
	Long: `Remove offstage-written hook scripts from the current repository's
.git/hooks/ directory. Hook files not written by offstage are left untouched.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		gd, err := getGitDir()
		if err != nil {
			return err
		}
		return hooks.UninstallLocal(gd)
	},
}

var hooksRunCmd = &cobra.Command{
	Use:   "run <hook-name> [args...]",
	Short: "Run an offstage hook (called by git hook scripts)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return hooks.Run(args[0], args[1:])
	},
}

// getGitDir returns the absolute path to the .git directory of the current repo.
func getGitDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	out, err := exec.Command("git", "rev-parse", "--git-dir").Output()
	if err != nil {
		return "", fmt.Errorf("not inside a git repo: %w", err)
	}
	dir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(cwd, dir)
	}
	return dir, nil
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
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	env, err := syncenv.Open(cwd, syncenv.WithManifest())
	if err != nil {
		return err
	}
	if err := syncGitExcludeIfEnabled(env.Resolved.RepoRoot, env.Manifest); err != nil {
		return err
	}

	if len(args) == 1 {
		currentStoreBranch := env.Resolved.ProjectID + "/" + env.Resolved.BranchName
		targetStoreBranch := env.Resolved.ProjectID + "/" + args[0]
		err = syncer.DiffBranches(env.Store, currentStoreBranch, targetStoreBranch)
	} else {
		storeBranch := env.Resolved.ProjectID + "/" + env.Resolved.BranchName
		err = syncer.DiffLocal(env.Store, env.Resolved.RepoRoot, manifest.EffectiveInclude(env.Manifest), env.Manifest.Exclude, storeBranch)
	}

	if err != nil {
		if !errors.Is(err, syncer.ErrHasDiff) {
			fmt.Fprintln(cmd.ErrOrStderr(), err)
		}
		os.Exit(syncer.ExitCode(err))
	}
	return nil
}

func syncGitExcludeIfEnabled(repoRoot string, cfg *manifest.ProjectConfig) error {
	if !cfg.GitExclude.AutoSync {
		return nil
	}
	_, err := gitexclude.Sync(repoRoot, manifest.EffectiveInclude(cfg))
	return err
}

func syncGitExcludeIfRequested(cmd *cobra.Command, repoRoot string, cfg *manifest.ProjectConfig, requested bool) error {
	if !requested && !cfg.GitExclude.AutoSync {
		return nil
	}
	result, err := gitexclude.Sync(repoRoot, manifest.EffectiveInclude(cfg))
	if err != nil {
		return err
	}
	if requested {
		printGitExcludeResult(cmd, result)
	}
	return nil
}

func printGitExcludeResult(cmd *cobra.Command, result *gitexclude.Result) {
	if result.PatternCount == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "nothing to exclude (no tracked patterns)")
		return
	}
	if result.Changed {
		fmt.Fprintf(cmd.OutOrStdout(), "synced %d patterns to %s\n", result.PatternCount, result.Path)
		return
	}
	fmt.Fprintf(cmd.OutOrStdout(), "git exclude already up to date (%d patterns)\n", result.PatternCount)
}

package cmd

import (
	"github.com/fathss/gx/internal/cli"
	"github.com/fathss/gx/internal/config"
	"github.com/fathss/gx/internal/workflow"

	"github.com/spf13/cobra"
)

var (
	syncContinue bool
	syncRebase   bool
	syncMerge    bool
	syncAbort    bool
	syncSkip     bool
)

var syncCmd = &cobra.Command{
	Use:   "sync [<remote> [<target_branch>]]",
	Short: "Synchronize the current branch with remote and base branch",
	Long: `Synchronize the current branch with its remote tracking branch and base branch.

Executes a safe two-phase synchronization:
  1. Reconciles the current branch with its remote counterpart
  2. Fast-forwards the local base branch and integrates it (rebase or merge)

If a target branch is specified, gx sync verifies that it matches the currently
checked-out branch and syncs only with that remote branch.

Uncommitted working tree changes are automatically stashed and restored.

Examples:
  gx sync                       Sync with remote feature branch then base branch
  gx sync origin                Sync using a specific remote
  gx sync origin feat/login     Sync only with remote tracking branch
  gx sync --rebase              Force rebase strategy (overrides config)
  gx sync --merge               Force merge strategy (overrides config)
  gx sync --continue            Resume paused sync after resolving conflicts
  gx sync --abort               Abort in-progress sync and restore state`,
	Args:  cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if syncAbort && syncContinue {
			return &cli.Error{
				Message: "Cannot specify both --abort and --continue.",
				Hint:    "Choose one action.",
			}
		}

		if syncRebase && syncMerge {
			return &cli.Error{
				Message: "Cannot specify both --rebase and --merge.",
				Hint:    "Choose one strategy.",
			}
		}

		// Issue 09a: Block conflicting flag combinations with --continue/--abort + --rebase/--merge
		if syncContinue && syncRebase {
			return &cli.Error{
				Message: "Cannot specify both --continue and --rebase.",
				Hint:    "The strategy is already determined by the in-progress operation.",
			}
		}
		if syncContinue && syncMerge {
			return &cli.Error{
				Message: "Cannot specify both --continue and --merge.",
				Hint:    "The strategy is already determined by the in-progress operation.",
			}
		}
		if syncAbort && syncRebase {
			return &cli.Error{
				Message: "Cannot specify both --abort and --rebase.",
				Hint:    "The strategy is already determined by the in-progress operation.",
			}
		}
		if syncAbort && syncMerge {
			return &cli.Error{
				Message: "Cannot specify both --abort and --merge.",
				Hint:    "The strategy is already determined by the in-progress operation.",
			}
		}

		// --skip is only meaningful with --continue (or alone during a paused rebase)
		if syncSkip {
			if syncAbort {
				return &cli.Error{
					Message: "Cannot specify both --skip and --abort.",
					Hint:    "Choose one action.",
				}
			}
			if syncMerge {
				return &cli.Error{
					Message: "--skip is not valid during a merge.",
					Hint:    "Use --continue to complete the merge, or --abort to cancel it.",
				}
			}
			if syncContinue {
				return &cli.Error{
					Message: "Cannot specify both --skip and --continue.",
					Hint:    "Use --skip to skip the current commit, or --continue to apply it.",
				}
			}
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		if len(args) >= 1 {
			cfg.Remote = args[0]
		}
		var targetBranch string
		if len(args) >= 2 {
			if syncContinue || syncSkip || syncAbort {
				return &cli.Error{
					Message: "Cannot specify a target branch with --continue, --skip, or --abort.",
					Hint:    "These flags operate on the in-progress sync for the current branch.",
				}
			}
			targetBranch = args[1]
		}

		return workflow.Sync(run, cfg, workflow.SyncOptions{
			Continue:     syncContinue,
			Rebase:       syncRebase,
			Merge:        syncMerge,
			Abort:        syncAbort,
			Skip:         syncSkip,
			TargetBranch: targetBranch,
		})
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.Flags().BoolVar(&syncContinue, "continue", false, "continue a paused sync (rebase or merge)")
	syncCmd.Flags().BoolVar(&syncRebase, "rebase", false, "use rebase strategy (overrides config)")
	syncCmd.Flags().BoolVar(&syncMerge, "merge", false, "use merge strategy (overrides config)")
	syncCmd.Flags().BoolVar(&syncAbort, "abort", false, "abort a paused sync (rebase or merge)")
	syncCmd.Flags().BoolVar(&syncSkip, "skip", false, "skip the current commit during --continue (rebase only)")
}

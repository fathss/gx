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
	Use:   "sync [<remote> [<base_branch>]]",
	Short: "Synchronize the current branch with the base branch",
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

		argsProvided := len(args) > 0
		if len(args) >= 1 {
			cfg.Remote = args[0]
		}
		if len(args) >= 2 {
			cfg.DefaultBranch = args[1]
		}

		return workflow.Sync(run, cfg, argsProvided, syncContinue, syncRebase, syncMerge, syncAbort, syncSkip)
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

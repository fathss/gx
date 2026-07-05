package cmd

import (
	"github.com/fathss/gx/internal/config"
	"github.com/fathss/gx/internal/workflow"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show repository status snapshot",
	Long: `Print a single, human-readable snapshot of the repo's current state.

Shows six sections in fixed order:
  1. Branch / HEAD state (with rebase/merge annotations)
  2. Staged files grouped by status type
  3. Unstaged tracked-file changes and untracked files
  4. Stash count
  5. Ahead/behind counts versus the upstream
  6. Recent commits (last 5)

gx status is read-only — it never mutates state, never fetches, and never
blocks on in-progress rebase or merge. Use --verbose to show the underlying
git commands that would otherwise be hidden.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		return workflow.Status(run, cfg)
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

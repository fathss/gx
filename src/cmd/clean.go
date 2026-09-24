package cmd

import (
	"github.com/fathss/gx/internal/config"
	"github.com/fathss/gx/internal/workflow"

	"github.com/spf13/cobra"
)

var cleanRemote bool

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Prune branches merged into the base branch",
	Long: `Prune branches that are already merged into the base branch.

Fetches from the configured remote first so the decision reflects current
remote state, then deletes:

  1. Local branches merged into <defaultBranch> (always)
  2. Remote-tracking refs merged into <remote>/<defaultBranch> (always)
  3. Remote branches (only with --remote, after confirmation)

Protected branches, the current branch, the default branch itself, and
<remote>/HEAD are never deleted. Deletion uses safe 'git branch -d' for
local branches so unmerged branches are refused. Remote deletion uses
'git push --delete' and affects every collaborator.

Examples:

  gx clean              # prune local + remote-tracking
  gx clean --remote     # also delete on the remote (prompts)`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		return workflow.Clean(run, cfg, cleanRemote)
	},
}

func init() {
	rootCmd.AddCommand(cleanCmd)
	cleanCmd.Flags().BoolVar(&cleanRemote, "remote", false, "also delete remote branches (prompts for confirmation)")
}

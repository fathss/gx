package cmd

import (
	"github.com/fathss/gx/internal/config"
	"github.com/fathss/gx/internal/workflow"

	"github.com/spf13/cobra"
)

var cleanRemote bool
var cleanYes bool

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Prune branches merged into the base branch",
	Long: `Prune branches that are already merged into the base branch.

Fetches from the configured remote first so the decision reflects current
remote state, then prompts before deleting:

  1. Local branches and remote-tracking refs merged into <defaultBranch>
  2. Remote branches (only with --remote), in a second prompt

Both prompts list the exact refs and default to No, so an empty answer or
closed stdin leaves everything untouched. Pass --yes / --y to answer both with yes
without being asked.

Protected branches, the current branch, the default branch itself, and
<remote>/HEAD are never deleted. Deletion uses safe 'git branch -d' for
local branches so unmerged branches are refused. Remote deletion uses
'git push --delete' and affects every collaborator.

Examples:

  gx clean              # prompt to prune local + remote-tracking
  gx clean --remote     # also prompt to delete on the remote
  gx clean --yes        # prune without prompting
  gx clean --yes --remote   # delete on the remote without prompting`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		return workflow.Clean(run, cfg, cleanRemote, cleanYes)
	},
}

func init() {
	rootCmd.AddCommand(cleanCmd)
	cleanCmd.Flags().BoolVar(&cleanRemote, "remote", false, "also prompt to delete remote branches")
	cleanCmd.Flags().BoolVarP(&cleanYes, "yes", "y", false, "skip the confirmation prompts")
}

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

Fetches with --prune from the configured remote first so the decision reflects
current remote state, then prompts before deleting:

  1. Local branches merged into the base branch
  2. Remote branches on origin (when --remote is specified)

Prompts list the exact branches and default to No, so an empty answer or
closed stdin leaves everything untouched. Pass --yes / -y to answer yes
without being asked.

Protected branches, the current branch, the default branch itself, and
<remote>/HEAD are never deleted. Deletion uses safe 'git branch -d' for
local branches so unmerged branches are refused. Remote deletion uses
'git push --delete' and affects every collaborator.

Examples:
  gx clean                   Prompt to prune local merged branches
  gx clean --remote          Prompt to prune local and remote branches
  gx clean --yes             Prune local branches without prompting
  gx clean --yes --remote    Prune local and remote branches without prompting`,
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

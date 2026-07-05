package cmd

import (
	"github.com/fathss/gx/internal/workflow"

	"github.com/spf13/cobra"
)

var (
	initRemote string
	initBranch string
	initForce  bool
)

var initCmd = &cobra.Command{
	Use:   "init [<remote> [<branch>]]",
	Short: "Initialize gx configuration for this repository",
	Long: `Detect repository settings and write .gx/config.

Auto-detects the remote and default branch, then writes them to .gx/config.
Override detection with --remote and --branch flags or positional arguments.

Examples:
  gx init                               Auto-detect and write config
  gx init upstream                      Force remote to "upstream"
  gx init upstream main                 Force both
  gx init --remote upstream --branch develop  Flag form
  gx init --force                       Overwrite existing config`,
	Annotations: map[string]string{"no_config": ""},
	Args:        cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		remote := initRemote
		branch := initBranch
		if len(args) >= 1 {
			remote = args[0]
		}
		if len(args) >= 2 {
			branch = args[1]
		}
		return workflow.Init(run, remote, branch, initForce)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVar(&initRemote, "remote", "", "remote name (overrides auto-detection)")
	initCmd.Flags().StringVar(&initBranch, "branch", "", "default branch name (overrides auto-detection)")
	initCmd.Flags().BoolVarP(&initForce, "force", "f", false, "overwrite existing .gx/config if it exists")
}

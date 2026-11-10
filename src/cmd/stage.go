package cmd

import (
	"github.com/fathss/gx/internal/config"
	"github.com/fathss/gx/internal/workflow"

	"github.com/spf13/cobra"
)

var (
	stageAll            bool
	stagePatch          bool
	stageAllowSensitive bool
	stageExclude        []string
)

var stageCmd = &cobra.Command{
	Use:     "stage [<files>...]",
	Aliases: []string{"add"},
	Short:   "Stage changes with sensitive file guardrails",
	Long: `Stage modified and untracked files into the Git index with safety checks.

Scans the working tree, shows a categorized preview of modified tracked files
and untracked files, and blocks files matching sensitive patterns (.env, *.pem, etc.)
unless overridden with --allow-sensitive.

Examples:
  gx stage                      Interactive staging with preview
  gx stage -a                   Stage all files immediately
  gx stage src/main.go          Stage specific files
  gx stage -p                   Select hunks interactively
  gx stage --exclude "*.log"    Stage excluding log files`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		return workflow.Stage(run, cfg, workflow.StageOptions{
			Files:           args,
			All:             stageAll,
			Patch:           stagePatch,
			AllowSensitive:  stageAllowSensitive,
			ExcludePatterns: stageExclude,
		})
	},
}

func init() {
	rootCmd.AddCommand(stageCmd)
	stageCmd.Flags().BoolVarP(&stageAll, "all", "a", false, "stage all modified and untracked files without prompting")
	stageCmd.Flags().BoolVarP(&stagePatch, "patch", "p", false, "interactively select hunks to stage")
	stageCmd.Flags().BoolVar(&stageAllowSensitive, "allow-sensitive", false, "override sensitive-pattern blocking")
	stageCmd.Flags().StringSliceVar(&stageExclude, "exclude", nil, "glob patterns to exclude (repeatable, comma-separated)")
}

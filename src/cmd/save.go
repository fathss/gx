package cmd

import (
	"strings"

	"github.com/fathss/gx/internal/cli"
	"github.com/fathss/gx/internal/config"
	"github.com/fathss/gx/internal/workflow"

	"github.com/spf13/cobra"
)

var (
	saveMsg            string
	savePatch          bool
	saveAllowSensitive bool
	saveAllowEmpty     bool
	saveExclude        []string
)

var saveCmd = &cobra.Command{
	Use:   "save [<files>...]",
	Short: "Stage changes and create a commit",
	Long: `Stage tracked-file changes and new files, then commit.

Without arguments, gx save scans the working tree, shows a categorized
preview of modified and untracked files, prompts for confirmation on
untracked files, and proceeds to commit.

Flags:
  -m <msg>              Commit with the given message (skip editor)
  --patch               Select hunks interactively (git add -p)
  --allow-sensitive     Override sensitive-pattern blocking
  --allow-empty         Allow committing with nothing staged
  --exclude <pattern>   Exclude files matching glob pattern from staging (repeatable, comma-separated)

Examples:
  gx save                            Interactive commit
  gx save -m "fix: login bug"        Quick commit with message
  gx save src/foo.go                 Stage and commit specific files
  gx save --patch                    Select hunks interactively
  gx save --allow-sensitive          Commit even with sensitive files`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Issue 11: Reject empty/whitespace-only commit message
		if cmd.Flags().Changed("message") {
			if strings.TrimSpace(saveMsg) == "" {
				return &cli.Error{
					Message: "Commit message cannot be empty.",
					Hint:    "Provide a meaningful message with -m, or omit -m to use the editor.",
				}
			}
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		return workflow.Save(run, cfg, args, saveMsg, savePatch, saveAllowSensitive, saveAllowEmpty, saveExclude)
	},
}

func init() {
	rootCmd.AddCommand(saveCmd)
	saveCmd.Flags().StringVarP(&saveMsg, "message", "m", "", "commit message (skip editor)")
	saveCmd.Flags().BoolVar(&savePatch, "patch", false, "interactively select hunks to stage")
	saveCmd.Flags().BoolVar(&saveAllowSensitive, "allow-sensitive", false, "override sensitive-pattern blocking")
	saveCmd.Flags().BoolVar(&saveAllowEmpty, "allow-empty", false, "allow committing with nothing staged")
	saveCmd.Flags().StringSliceVar(&saveExclude, "exclude", nil, "glob patterns for files to exclude from staging (repeatable, comma-separated)")
}

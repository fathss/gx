package cmd

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/fathss/gx/internal/runner"
	"github.com/fathss/gx/internal/workflow"

	"github.com/spf13/cobra"
)

var (
	verbose          bool
	run              *runner.Runner
	annotateBuiltins sync.Once
)

var rootCmd = &cobra.Command{
	Use:   "gx",
	Short: "gx — opinionated git workflows in single commands",
	Long: `gx wraps common git workflows into single commands that follow best practices.
Instead of running 4-7 git commands manually, gx does the right sequence for you.

Use --verbose to see every git command gx runs under the hood.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		annotateBuiltins.Do(func() {
			var annotateSubtree func(*cobra.Command)
			annotateSubtree = func(c *cobra.Command) {
				if c.Annotations == nil {
					c.Annotations = make(map[string]string)
				}
				c.Annotations["no_config"] = ""
				c.Annotations["no_repo"] = ""
				for _, sub := range c.Commands() {
					annotateSubtree(sub)
				}
			}
			for _, child := range cmd.Root().Commands() {
				switch child.Name() {
				case "help", "completion":
					annotateSubtree(child)
				}
			}
		})

		if _, ok := cmd.Annotations["no_repo"]; !ok {
			if err := workflow.RequireRepository(run); err != nil {
				return err
			}
		}

		if _, ok := cmd.Annotations["no_config"]; ok {
			return nil
		}
		return workflow.RequireConfig()
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "print every git command gx runs")
	cobra.OnInitialize(initRunner)
}

func initRunner() {
	run = runner.New(verbose)
	run.WarnFn = func(msg, hint string) {
		fmt.Fprintf(os.Stderr, "⚠ %s\n", msg)
		if hint != "" {
			for _, line := range strings.Split(hint, "\n") {
				fmt.Fprintf(os.Stderr, "  %s\n", line)
			}
		}
	}
}
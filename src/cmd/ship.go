package cmd

import (
	"github.com/fathss/gx/internal/config"
	"github.com/fathss/gx/internal/workflow"

	"github.com/spf13/cobra"
)

var (
	shipForce bool
	shipNoPR  bool
)

var shipCmd = &cobra.Command{
	Use:   "ship",
	Short: "Push the current branch and print the PR creation URL",
	Long: `Push the current branch to the configured remote, set upstream if
needed, and print the PR creation URL.

Refuses to push to protected branches (main, master, develop by default).
Detects diverged history and requires --force before force-pushing.
Uses --force-with-lease rather than bare --force.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		return workflow.Ship(run, cfg, shipForce, shipNoPR)
	},
}

func init() {
	rootCmd.AddCommand(shipCmd)
	shipCmd.Flags().BoolVar(&shipForce, "force", false, "allow pushing when local and remote history have diverged")
	shipCmd.Flags().BoolVar(&shipNoPR, "no-pr", false, "skip PR URL lookup/printing after push")
}

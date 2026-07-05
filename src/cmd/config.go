package cmd

import (
	"fmt"

	"github.com/fathss/gx/internal/cli"
	"github.com/fathss/gx/internal/config"

	"github.com/spf13/cobra"
)

var (
	configOverwrite bool
)

var configCmd = &cobra.Command{
	Use:   "config [<key> [<value>]]",
	Short: "Get or set persistent configuration values",
	Annotations: map[string]string{"no_config": ""},
	Long: `Manage the .gx/config configuration file.

Keys:
  remote                  Remote name (default: origin)
  defaultBranch           Branch to sync against (default: develop)
  syncStrategy            Rebase or merge strategy (default: rebase)
  sensitivePatterns       Additional sensitive file patterns
  protectedBranches       Branches protected from deletion (default: main, master, develop)

Examples:
  gx config                                          Show all values
  gx config list                                     Show all values
  gx config remote                                   Print current remote
  gx config remote upstream                          Set remote to upstream
  gx config sensitivePatterns .env .gitignore           Append patterns
  gx config sensitivePatterns .env .gitignore --overwrite  Overwrite list
  gx config protectedBranches main release              Append branches
  gx config protectedBranches main release --overwrite  Overwrite list`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		switch len(args) {
		case 0:
			// Issue 21: 0 args + --overwrite = sensitivePatterns/protectedBranches error
			if configOverwrite {
				return &cli.Error{
					Message: "sensitivePatterns or protectedBranches: no patterns specified",
					Hint:    "Usage: gx config sensitivePatterns <pattern> [<pattern>...] [--overwrite] or gx config protectedBranches <branch> [<branch>...] [--overwrite]",
				}
			}
			return printConfig()
		case 1:
			// Issue 20: gx config list — positional subcommand
			if args[0] == "list" {
				return printConfig()
			}
			// Issue 21: sensitivePatterns/protectedBranches --overwrite with 0 pattern args
			if (args[0] == "sensitivePatterns" || args[0] == "protectedBranches") && configOverwrite {
				return &cli.Error{
					Message: args[0] + ": no patterns specified",
					Hint:    "Usage: gx config " + args[0] + " <value> [<value>...] [--overwrite]",
				}
			}
			// Issue 18: --overwrite only valid for sensitivePatterns and protectedBranches
			if configOverwrite {
				return &cli.Error{
					Message: "--overwrite is only valid for sensitivePatterns and protectedBranches",
					Hint:    "Remove --overwrite or use it with: gx config sensitivePatterns <patterns>... --overwrite or gx config protectedBranches <branches>... --overwrite",
				}
			}
			val, err := config.Get(args[0])
			if err != nil {
				return &cli.Error{
					Message: fmt.Sprintf("Failed to get config key: %s.", args[0]),
				}
			}
			if val == "" {
				fmt.Println("(not set)")
			} else {
				fmt.Println(val)
			}
			return nil
		default:
			// Issue 18: --overwrite only valid for sensitivePatterns and protectedBranches
			if configOverwrite && args[0] != "sensitivePatterns" && args[0] != "protectedBranches" {
				return &cli.Error{
					Message: "--overwrite is only valid for sensitivePatterns and protectedBranches",
					Hint:    "Remove --overwrite or use it with: gx config sensitivePatterns <patterns>... --overwrite or gx config protectedBranches <branches>... --overwrite",
				}
			}
			// Issue 20: gx config list <extra> — error
			if args[0] == "list" {
				return &cli.Error{
					Message: "config list takes no arguments",
				}
			}
			if args[0] == "sensitivePatterns" {
				if configOverwrite {
					return config.SetSensitivePatterns(args[1:])
				}
				return config.AppendSensitivePatterns(args[1:])
			}
			if args[0] == "protectedBranches" {
				if configOverwrite {
					return config.SetProtectedBranches(args[1:])
				}
				return config.AppendProtectedBranches(args[1:])
			}
			if len(args) > 2 {
				return &cli.Error{
					Message: fmt.Sprintf("config %s takes at most 1 value", args[0]),
				}
			}
			if err := config.Set(args[0], args[1]); err != nil {
				return &cli.Error{
					Message: err.Error(),
				}
			}
			return nil
		}
	},
}

func printConfig() error {
	for _, key := range []string{"remote", "defaultBranch", "syncStrategy", "sensitivePatterns", "protectedBranches"} {
		val, err := config.Get(key)
		if err != nil {
			return &cli.Error{
				Message: fmt.Sprintf("Failed to load config: %s.", err),
			}
		}
		if val == "" {
			val = "(not set)"
		}
		fmt.Printf("%-20s %s\n", key, val)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.Flags().BoolVar(&configOverwrite, "overwrite", false, "overwrite sensitivePatterns list instead of appending")
}

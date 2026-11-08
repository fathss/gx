package cmd

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/fathss/gx/internal/ui"
	"github.com/spf13/cobra"
)

var (
	uiForceColor bool
	uiDisableColor bool
	uiForceTTY   bool
)

var internalUICmd = &cobra.Command{
	Use:    "__ui",
	Short:  "Internal UI testing utilities",
	Hidden: true,
	Annotations: map[string]string{
		"no_repo":   "",
		"no_config": "",
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("subcommand required: color, spinner, frames")
		}

		if uiForceColor {
			ui.SetColorEnabled(true)
		} else if uiDisableColor {
			ui.SetColorEnabled(false)
		}

		switch args[0] {
		case "color":
			if len(args) < 3 {
				return fmt.Errorf("usage: gx __ui color <style> <text>")
			}
			style := strings.ToLower(args[1])
			text := args[2]

			switch style {
			case "white":
				fmt.Print(ui.White(text))
			case "dim":
				fmt.Print(ui.Dim(text))
			case "success":
				fmt.Print(ui.Success(text))
			case "danger":
				fmt.Print(ui.Danger(text))
			case "warning":
				fmt.Print(ui.Warning(text))
			case "checkmark":
				fmt.Print(ui.Checkmark())
			case "cross":
				fmt.Print(ui.Cross())
			case "warningsymbol":
				fmt.Print(ui.WarningSymbol())
			default:
				return fmt.Errorf("unknown color style: %s", style)
			}
			return nil

		case "frames":
			for i, f := range ui.DefaultSlashFrames {
				if i > 0 {
					fmt.Print(",")
				}
				fmt.Print(f)
			}
			fmt.Println()
			return nil

		case "spinner":
			if len(args) < 2 {
				return fmt.Errorf("usage: gx __ui spinner <tick|update|success|fail> [args...]")
			}
			action := args[1]
			msg := "Processing"
			if len(args) >= 3 {
				msg = args[2]
			}

			opts := []ui.SpinnerOption{}
			if uiForceTTY {
				opts = append(opts, ui.WithTTY(true))
			}

			sp := ui.NewSpinner(msg, opts...)

			switch action {
			case "tick":
				var buf bytes.Buffer
				testSp := ui.NewSpinner("Ticking", ui.WithWriter(&buf), ui.WithTTY(true))
				testSp.Start()
				testSp.Tick()
				testSp.Tick()
				testSp.Stop()
				// Print buffer output with escaped representation so tests can inspect
				fmt.Print(buf.String())
				return nil

			case "update":
				var buf bytes.Buffer
				testSp := ui.NewSpinner("Initial message", ui.WithWriter(&buf), ui.WithTTY(true))
				testSp.Start()
				testSp.UpdateMessage("Updated message")
				testSp.Stop()
				fmt.Print(buf.String())
				return nil

			case "success":
				sp.Start()
				time.Sleep(20 * time.Millisecond)
				sp.StopSuccess(msg)
				return nil

			case "fail":
				hint := ""
				if len(args) >= 4 {
					hint = args[3]
				}
				sp.Start()
				time.Sleep(20 * time.Millisecond)
				sp.StopFail(msg, hint)
				return nil

			default:
				return fmt.Errorf("unknown spinner action: %s", action)
			}

		default:
			return fmt.Errorf("unknown command: %s", args[0])
		}
	},
}

func init() {
	internalUICmd.Flags().BoolVar(&uiForceColor, "force-color", false, "force color output")
	internalUICmd.Flags().BoolVar(&uiDisableColor, "disable-color", false, "disable color output")
	internalUICmd.Flags().BoolVar(&uiForceTTY, "tty", false, "force TTY spinner mode")
	rootCmd.AddCommand(internalUICmd)
}

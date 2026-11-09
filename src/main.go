package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/fathss/gx/cmd"
	"github.com/fathss/gx/internal/cli"
	"github.com/fathss/gx/internal/ui"
)

func main() {
	if err := cmd.Execute(); err != nil {
		printError(err)
		os.Exit(1)
	}
}

func printError(err error) {

	var cliErr *cli.Error
	if errors.As(err, &cliErr) {
		fmt.Fprintf(os.Stderr, "%s\n", ui.Danger(cliErr.Message))

		if cliErr.Hint != "" {
			for _, line := range strings.Split(cliErr.Hint, "\n") {
				fmt.Fprintf(os.Stderr, "hint: %s\n", line)
			}
		}

		return
	}

	fmt.Fprintf(os.Stderr, "%s\n", ui.Danger(err.Error()))
}
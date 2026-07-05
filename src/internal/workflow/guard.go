package workflow

import (
	"github.com/fathss/gx/internal/cli"
	"github.com/fathss/gx/internal/config"
	"github.com/fathss/gx/internal/git"
	"github.com/fathss/gx/internal/runner"
)

// RequireRepository returns an error when not inside a git repository.
func RequireRepository(run *runner.Runner) error {
	if !git.IsRepository(run) {
		return &cli.Error{
			Message: "Not inside a Git repository.",
			Hint:    "Run gx inside a cloned repository.",
		}
	}
	return nil
}

// RequireConfig returns an error if no .gx/config file exists.
func RequireConfig() error {
	if !config.Exists() {
		return &cli.Error{
			Message: "No existing configuration",
			Hint:    "Run gx init to start using gx",
		}
	}
	return nil
}

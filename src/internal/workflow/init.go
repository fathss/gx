package workflow

import (
	"fmt"

	"github.com/fathss/gx/internal/cli"
	"github.com/fathss/gx/internal/config"
	"github.com/fathss/gx/internal/git"
	"github.com/fathss/gx/internal/runner"
)

// Init detects repository settings and writes .gx/config.
// remoteOverride and branchOverride bypass auto-detection when non-empty.
// If force is true, overwrites any existing configuration.
func Init(run *runner.Runner, remoteOverride, branchOverride string, force bool) error {
	if config.Exists() && !force {
		return &cli.Error{
			Message: "Configuration already exists.",
			Hint:    "Edit .gx/config directly or use gx config to update values.",
		}
	}

	remote := remoteOverride
	if remote == "" {
		var err error
		remote, err = git.DetectRemote(run)
		if err != nil {
			return &cli.Error{
				Message: "Failed to detect remote.",
				Hint:    fmt.Sprintf("(%s)\nSpecify it explicitly: gx init <remote>", err),
			}
		}
	}

	defaultBranch := branchOverride
	if defaultBranch == "" {
		var err error
		defaultBranch, err = git.DetectDefaultBranch(run, remote)
		if err != nil {
			return &cli.Error{
				Message: "Failed to detect default branch.",
				Hint:    fmt.Sprintf("(%s)\nSpecify it explicitly: gx init <remote> <branch>", err),
			}
		}
	}

	cfg := &config.Config{
		Remote:            remote,
		DefaultBranch:     defaultBranch,
		SyncStrategy:      "rebase",
		SensitivePatterns:  config.DefaultSensitivePatterns,
		ProtectedBranches: config.DefaultProtectedBranches,
	}

	if err := config.Save(cfg); err != nil {
		return &cli.Error{
			Message: "Failed to write configuration.",
			Hint:    fmt.Sprintf("(%s)\nCheck permissions on the .gx/ directory.", err),
		}
	}

	fmt.Printf("Wrote .gx/config\n")
	fmt.Printf("  remote:            %s\n", cfg.Remote)
	fmt.Printf("  defaultBranch:     %s\n", cfg.DefaultBranch)
	fmt.Printf("  syncStrategy:      %s\n", cfg.SyncStrategy)
	fmt.Printf("  sensitivePatterns: %s\n", cfg.SensitivePatterns)

	return nil
}

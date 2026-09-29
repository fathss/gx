package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/fathss/gx/internal/cli"
	"github.com/fathss/gx/internal/config"
	"github.com/fathss/gx/internal/forge"
	"github.com/fathss/gx/internal/git"
	"github.com/fathss/gx/internal/runner"
)

// Ship pushes the current branch to the configured remote and prints a PR URL.
func Ship(run runner.Executor, cfg *config.Config, force bool, noPR bool) error {
	// 1. Check for in-progress rebase or merge (before branch check — rebase
	//    pauses leave HEAD detached)
	if err := requireNoInProgress(inspect(run)); err != nil {
		return err
	}

	// 2. Determine current branch
	current, err := git.CurrentBranch(run)
	if err != nil {
		return &cli.Error{
			Message: "Failed to determine current branch.",
		}
	}
	if current == "" {
		return &cli.Error{
			Message: "Cannot ship from a detached HEAD.",
			Hint:    "Create a branch first: git checkout -b <name>.",
		}
	}

	// 3. Protected branch check (not overridable by --force)
	for _, protected := range cfg.ProtectedBranches {
		if current == protected {
			return &cli.Error{
				Message: fmt.Sprintf("Refusing to push directly to protected branch %q.", current),
				Hint:    "Open a feature branch with git checkout -b <name> or cut a release with gx tag.",
			}
		}
	}

	// 4. Fetch remote to update remote-tracking refs, then check divergence
	//    against the configured remote — gx's one notion of upstream.
	if err := git.Fetch(run, cfg.Remote); err != nil {
		return &cli.Error{
			Message: fmt.Sprintf("Failed to fetch branch %q from %q.", current, cfg.Remote),
			Hint:    "Check the error above; verify the remote with `git remote -v`.",
		}
	}

	div, err := git.CheckDivergence(run, cfg.Remote, current)
	if err != nil {
		return &cli.Error{
			Message: "Failed to check divergence status.",
		}
	}

	var pushFn func() error

	switch {
	case !div.Tracking:
		// Remote-tracking ref doesn't exist yet — first push sets upstream
		pushFn = func() error {
			return git.PushSetUpstream(run, cfg.Remote, current)
		}

	case div.Behind > 0 && div.Ahead == 0:
		// Local is behind remote — pointless to push
		return &cli.Error{
			Message: fmt.Sprintf("Remote has commits not on local for %q.\n  remote: %d commit(s) ahead", current, div.Behind),
			Hint:    "Pull or rebase first (gx sync).",
		}

	case div.Behind > 0 && div.Ahead > 0:
		// Diverged
		if !force {
			return &cli.Error{
				Message: fmt.Sprintf("Local and remote history have diverged for %q.\n  local:  %d commit(s) not on remote\n  remote: %d commit(s) not on local", current, div.Ahead, div.Behind),
				Hint:    "Rebase or merge first (gx sync), or re-run with --force if you intend to overwrite remote history.",
			}
		}
		pushFn = func() error {
			return git.PushForceWithLease(run, cfg.Remote, current)
		}

	default:
		// Fast-forward (ahead >= 0, behind == 0) or up-to-date (both 0)
		pushFn = func() error {
			return git.Push(run, cfg.Remote, current)
		}
	}

	// 5. Push
	if err := pushFn(); err != nil {
		hint := "Check the error above."
		if git.IsNonFastForward(err) {
			hint = "Someone may have pushed since your last fetch — run gx sync and retry."
		}
		return &cli.Error{
			Message: "Push rejected by remote.",
			Hint:    hint,
		}
	}

	fmt.Printf("✔ Pushed %s → %s\n", current, cfg.Remote)

	// 6. PR URL (unless --no-pr)
	if noPR {
		return nil
	}

	remoteURL, err := git.RemoteURL(run, cfg.Remote)
	if err != nil {
		return nil // push already succeeded
	}

	f, err := forge.NewForge(remoteURL)
	if err != nil {
		run.Warnf("Could not detect a known git host for PR URL.", fmt.Sprintf("Remote: %s", remoteURL))
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	prURL, err := f.ExistingPRURL(ctx, cfg.DefaultBranch, current)
	if err != nil {
		// API failure — fall through to creation URL
		prURL = ""
	}
	if prURL == "" {
		prURL = f.PRCreateURL(cfg.DefaultBranch, current)
	}
	fmt.Printf("→ %s\n", prURL)

	return nil
}

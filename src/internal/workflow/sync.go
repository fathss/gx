package workflow

import (
	"fmt"
	"strings"

	"github.com/fathss/gx/internal/cli"
	"github.com/fathss/gx/internal/config"
	"github.com/fathss/gx/internal/git"
	"github.com/fathss/gx/internal/runner"
)

// SyncOptions carries the invocation mode for gx sync. Named fields so call
// sites and flag-conflict validation read as one structured interface.
type SyncOptions struct {
	Continue     bool
	Rebase       bool
	Merge        bool
	Abort        bool
	Skip         bool
	TargetBranch string
}

func isProtectedBranch(cfg *config.Config, branch string) bool {
	if branch == "" || branch == cfg.DefaultBranch {
		return true
	}
	for _, p := range cfg.ProtectedBranches {
		if branch == p {
			return true
		}
	}
	return false
}

func Sync(run runner.Executor, cfg *config.Config, opts SyncOptions) error {
	// 1. Determine current branch and classify repository state. The gx
	// ownership marker is authoritative when present; the branch it names is
	// used to scope every stash lookup (HEAD is detached during a rebase).
	current, err := git.CurrentBranch(run)
	if err != nil {
		return &cli.Error{
			Message: "Failed to determine current branch.",
		}
	}

	// Pre-flight guard: if a target branch was explicitly specified, it must
	// match the currently checked out branch.
	if opts.TargetBranch != "" && opts.TargetBranch != current {
		return &cli.Error{
			Message: fmt.Sprintf("You are currently on %q, but specified %q.", current, opts.TargetBranch),
			Hint:    fmt.Sprintf("Switch branches first:\n  git checkout %s\n  gx sync", opts.TargetBranch),
		}
	}

	release, err := acquireSyncLock(run)
	if err != nil {
		return err
	}
	defer release()
	verdict, stateBranch, hasMarker := inspectForSync(run, current)
	// A marker with no paused operation is stale. If it belongs to another
	// branch (the user aborted a gx rebase manually and switched branches),
	// leave that branch's stash untouched; restoring it here would leak one
	// branch's work into another. Clear only the marker and sync as Q1.
	if verdict == q2OrphanStash && hasMarker && stateBranch != current {
		if err := git.ClearGXSyncState(run); err != nil {
			run.Warnf("Failed to clear a stale in-progress sync marker.", fmt.Sprintf("Error: %v", err))
		}
		verdict = q1Idle
		stateBranch = current
		hasMarker = false
	}
	sessionGit := newProdSyncGit(run, stateBranch)

	// --abort: abort in-progress rebase or merge, settle the gx stash if present.
	// Only works for gx-orchestrated operations (marker or branch-scoped stash).
	// Manual rebases/merges block with a message telling the user to use git.
	if opts.Abort {
		switch verdict {
		case q3ManualRebase:
			// Q3: manual rebase — gx sync only manages its own operations
			return &cli.Error{
				Message: "A rebase is already in progress.",
				Hint:    "gx sync only manages its own sync operations.\nUse git rebase --abort directly.",
			}

		case q4GXRebase:
			// Q4: gx-orchestrated rebase — abort, then settle
			run.Info("Aborting rebase...")
			return abortWithStash(run, sessionGit, git.RebaseAbort)

		case q5ManualMerge:
			// Q5: manual merge — gx sync only manages its own operations
			return &cli.Error{
				Message: "A merge is already in progress.",
				Hint:    "gx sync only manages its own sync operations.\nUse git merge --abort directly.",
			}

		case q6GXMerge:
			// Q6: gx-orchestrated merge — abort, then settle
			run.Info("Aborting merge...")
			return abortWithStash(run, sessionGit, git.MergeAbort)

		default:
			// Q1 or Q2: nothing to abort. A marker with no paused operation is
			// stale (e.g. the user aborted manually); clear it.
			if hasMarker {
				if err := sessionGit.ClearState(); err != nil {
					sessionGit.Warnf("Failed to clear the in-progress sync state.", fmt.Sprintf("Error: %v", err))
				}
			}
			return &cli.Error{
				Message: "Nothing to abort.",
				Hint:    "Run gx sync to start a new sync.",
			}
		}
	}

	// 3. State machine for rebase/merge + stash interactions — classified
	// once by inspectForSync() (the Q-table lives in repostate.go); this
	// switch is the interpretation for gx sync.
	//
	//    +---+------------------+------------------+---------------+-----------------------------------------+
	//    |   | RebaseInProgress | MergeInProgress  | GXOwned       | Behavior                                |
	//    +---+------------------+------------------+---------------+-----------------------------------------+
	//    | 1 | false            | false            | false         | normal flow                             |
	//    | 2 | false            | false            | true          | restore orphan, proceed                 |
	//    | 3 | true             | false            | false         | block: manual rebase (--continue denied) |
	//    | 4 | true             | false            | true          | block: gx rebase (--continue allowed)    |
	//    | 5 | false            | true             | false         | block: manual merge (--continue denied)  |
	//    | 6 | false            | true             | true          | block: gx merge (--continue allowed)     |
	//    +---+------------------+------------------+---------------+-----------------------------------------+
	switch verdict {
	case q4GXRebase:
		// Q4: gx-orchestrated rebase — --continue resumes, --skip skips, plain blocks
		if opts.Skip {
			return autoSkipSync(run, sessionGit, cfg)
		}
		if opts.Continue {
			return autoContinueSync(run, sessionGit, cfg)
		}
		return &cli.Error{
			Message: "A rebase is already in progress.",
			Hint:    "Run gx sync --continue to resume, gx sync --skip to skip, or gx sync --abort to cancel.",
		}

	case q6GXMerge:
		// Q6: gx-orchestrated merge — --continue resumes, plain blocks
		if opts.Skip {
			return &cli.Error{
				Message: "--skip is not valid during a merge.",
				Hint:    "Use git merge --continue or --abort directly.",
			}
		}
		if opts.Continue {
			return autoContinueMergeSync(run, sessionGit, cfg)
		}
		return &cli.Error{
			Message: "A merge is already in progress.",
			Hint:    "Run gx sync --continue to resume, or gx sync --abort to cancel.",
		}

	case q3ManualRebase:
		// Q3: manual rebase — gx sync only manages its own operations
		if opts.Skip {
			return &cli.Error{
				Message: "A rebase is already in progress.",
				Hint:    "gx sync only manages its own sync operations.\nUse git rebase --skip directly.",
			}
		}
		return &cli.Error{
			Message: "A rebase is already in progress.",
			Hint:    "gx sync only manages its own sync operations.\nUse git rebase --continue, --skip, or --abort directly.",
		}

	case q5ManualMerge:
		// Q5: manual merge — gx sync only manages its own operations
		return &cli.Error{
			Message: "A merge is already in progress.",
			Hint:    "gx sync only manages its own sync operations.\nUse git merge --continue or --abort directly.",
		}

	case q1Idle, q2OrphanStash:
		// Detached HEAD with no paused operation: refuse before touching any
		// stash. (A paused gx rebase is detached too, but classifies Q4.)
		if current == "" {
			if hasMarker {
				if err := sessionGit.ClearState(); err != nil {
					sessionGit.Warnf("Failed to clear the in-progress sync state.", fmt.Sprintf("Error: %v", err))
				}
			}
			return &cli.Error{
				Message: "Detached HEAD is not supported.",
				Hint:    "Checkout a branch first.",
			}
		}
		if opts.Continue || opts.Skip {
			// --continue or --skip but nothing paused. Clear a stale marker so
			// the next run starts clean.
			if hasMarker {
				if err := sessionGit.ClearState(); err != nil {
					sessionGit.Warnf("Failed to clear the in-progress sync state.", fmt.Sprintf("Error: %v", err))
				}
			}
			return &cli.Error{
				Message: "Nothing to continue. Rebase appears to have completed already.",
				Hint:    "Run git status to verify the state, then gx sync to start a fresh sync.",
			}
		}
		if verdict == q2OrphanStash {
			// Q2: orphaned gx restore — settle the adopted session, then proceed
			if err := resume(sessionGit).settle(settleRestore); err != nil {
				return err
			}
		}
	}

	// 4. Acquire the session: stash the dirty working tree (skipped when clean).
	session, err := begin(sessionGit, current)
	if err != nil {
		return err
	}

	// 5–8. Act phase. Every exit settles through the session: an error
	// restores the stash, a conflict pause keeps it, success commits it.
	err = session.run(func() (settle, error) {
		if !git.RemoteExists(run, cfg.Remote) {
			return settleRestore, &cli.Error{
				Message: fmt.Sprintf("Remote '%s' not found.", cfg.Remote),
				Hint:    "Run `git remote -v` to list available remotes, then `gx config remote <name>` to update.",
			}
		}
		run.State(fmt.Sprintf("Fetching %s...", cfg.Remote))
		if err := git.Fetch(run, cfg.Remote); err != nil {
			return settleRestore, &cli.Error{
				Message: "Failed to fetch remote.",
				Hint:    "Check the error above; verify the remote with `git remote -v`.",
			}
		}

		// Issue 06: Check for stale remote default branch after fetch
		detectedBranch := git.RemoteHEAD(run, cfg.Remote)
		if detectedBranch != "" && detectedBranch != cfg.DefaultBranch {
			run.Warnf(
				fmt.Sprintf("Remote default branch appears to be '%s' but gx is configured for '%s'.", detectedBranch, cfg.DefaultBranch),
				fmt.Sprintf("Update config with: gx config defaultBranch %s", detectedBranch),
			)
		}

		// Use flag override if provided, otherwise config
		strategy := cfg.SyncStrategy
		switch {
		case opts.Rebase:
			strategy = "rebase"
		case opts.Merge:
			strategy = "merge"
		}

		integrate := func(target, ontoLabel string) (settle, error) {
			if err := sessionGit.MarkState(); err != nil {
				return settleRestore, &cli.Error{
					Message: "Failed to record the in-progress sync state.",
					Hint:    "Check that the repository is writable, then run gx sync again.",
				}
			}
			switch strategy {
			case "merge":
				run.State(fmt.Sprintf("Merging %s...", target))
				if err := git.Merge(run, target); err != nil {
					if git.IsMergeInProgress(run) {
						msg := "Merge stopped because of conflicts."
						files, filesErr := git.ConflictedFiles(run)
						switch {
						case filesErr != nil:
							msg += " The conflicted files could not be listed."
						case len(files) > 0:
							msg += fmt.Sprintf(" Conflicted files: %s", strings.Join(files, ", "))
						}
						hint := "Resolve the conflicts, stage the files, then run gx sync --continue."
						if session.acquired {
							hint += " Your changes were stashed and will be restored automatically."
						}
						return settlePause, &cli.Error{
							Message: msg,
							Hint:    hint,
						}
					}
					return settleRestore, &cli.Error{
						Message: "Merge failed.",
					}
				}
			default:
				run.State(fmt.Sprintf("Rebasing onto %s...", ontoLabel))
				if err := git.Rebase(run, target); err != nil {
					if git.IsRebaseInProgress(run) {
						msg := "Rebase stopped because of conflicts."
						if info := git.CurrentRebasePatchInfo(run); info != "" {
							msg = fmt.Sprintf("Rebase stopped because of conflicts while applying commit %s.", info)
						}
						files, filesErr := git.ConflictedFiles(run)
						switch {
						case filesErr != nil:
							msg += " The conflicted files could not be listed."
						case len(files) > 0:
							msg += fmt.Sprintf(" Conflicted files: %s", strings.Join(files, ", "))
						}
						hint := "Resolve the conflicts, stage the files, then run gx sync --continue."
						if session.acquired {
							hint += " Your changes were stashed and will be restored automatically."
						}
						return settlePause, &cli.Error{
							Message: msg,
							Hint:    hint,
						}
					}
					return settleRestore, &cli.Error{
						Message: "Rebase failed.",
					}
				}
			}
			return settleCommit, nil
		}

		// Case 1: Target-only mode (opts.TargetBranch == current)
		if opts.TargetBranch != "" {
			if isProtectedBranch(cfg, current) {
				baseRef := git.Qualify(cfg.Remote, current)
				if !git.RemoteBranchExists(run, cfg.Remote, current) {
					return settleRestore, &cli.Error{
						Message: fmt.Sprintf("Remote branch '%s' not found.", baseRef),
						Hint:    "Has it been deleted? Check the remote repository.",
					}
				}
				run.State(fmt.Sprintf("Fast-forwarding %s...", current))
				if err := git.FastForward(run, cfg.Remote, current); err != nil {
					return settleRestore, &cli.Error{
						Message: fmt.Sprintf("Local branch '%s' has diverged from '%s'.", current, baseRef),
						Hint:    "The base branch should not have local commits.\nMove them to a feature branch:\n  git checkout -b fix/description\n  git branch -f " + current + " " + baseRef + "\nOr rebase and force-push:\n  git rebase " + baseRef + "\n  git push --force-with-lease",
					}
				}
				return settleCommit, nil
			}

			// Feature branch target-only
			if !git.RemoteBranchExists(run, cfg.Remote, current) {
				run.Info(fmt.Sprintf("Remote branch '%s/%s' does not exist yet.", cfg.Remote, current))
				return settleCommit, nil
			}
			div, err := git.CheckDivergence(run, cfg.Remote, current)
			if err != nil {
				return settleRestore, &cli.Error{
					Message: "Failed to check divergence status.",
				}
			}
			if div.Behind > 0 && div.Ahead == 0 {
				run.State(fmt.Sprintf("Fast-forwarding %s from %s/%s...", current, cfg.Remote, current))
				if err := git.FastForward(run, cfg.Remote, current); err != nil {
					return settleRestore, &cli.Error{
						Message: fmt.Sprintf("Failed to fast-forward %s.", current),
					}
				}
				return settleCommit, nil
			}
			if div.Behind > 0 && div.Ahead > 0 {
				remoteRef := git.Qualify(cfg.Remote, current)
				return integrate(remoteRef, remoteRef)
			}
			return settleCommit, nil
		}

		// Case 2: Normal mode on protected/base branch
		if isProtectedBranch(cfg, current) {
			baseRef := git.Qualify(cfg.Remote, current)
			if !git.RemoteBranchExists(run, cfg.Remote, current) {
				return settleRestore, &cli.Error{
					Message: fmt.Sprintf("Remote branch '%s' not found.", baseRef),
					Hint:    "Has it been deleted? Check the remote repository.",
				}
			}
			run.State(fmt.Sprintf("Fast-forwarding %s...", current))
			if err := git.FastForward(run, cfg.Remote, current); err != nil {
				return settleRestore, &cli.Error{
					Message: fmt.Sprintf("Local branch '%s' has diverged from '%s'.", current, baseRef),
					Hint:    "The base branch should not have local commits.\nMove them to a feature branch:\n  git checkout -b fix/description\n  git branch -f " + current + " " + baseRef + "\nOr rebase and force-push:\n  git rebase " + baseRef + "\n  git push --force-with-lease",
				}
			}
			return settleCommit, nil
		}

		// Case 3: Normal mode on feature branch — Two-Phase Sync
		// Phase 1: Reconcile with remote feature branch if it exists
		if git.RemoteBranchExists(run, cfg.Remote, current) {
			div, err := git.CheckDivergence(run, cfg.Remote, current)
			if err != nil {
				return settleRestore, &cli.Error{
					Message: "Failed to check divergence status.",
				}
			}
			if div.Behind > 0 && div.Ahead == 0 {
				run.State(fmt.Sprintf("Fast-forwarding %s from %s/%s...", current, cfg.Remote, current))
				if err := git.FastForward(run, cfg.Remote, current); err != nil {
					return settleRestore, &cli.Error{
						Message: fmt.Sprintf("Failed to fast-forward %s.", current),
					}
				}
			} else if div.Behind > 0 && div.Ahead > 0 {
				remoteRef := git.Qualify(cfg.Remote, current)
				outcome, err := integrate(remoteRef, remoteRef)
				if err != nil || outcome != settleCommit {
					return outcome, err
				}
			}
		}

		// Phase 2: Switch to base branch, fast-forward, return to original and integrate
		if !git.LocalBranchExists(run, cfg.DefaultBranch) {
			return settleRestore, &cli.Error{
				Message: fmt.Sprintf("Local branch '%s' does not exist.", cfg.DefaultBranch),
				Hint:    fmt.Sprintf("Create it with: git checkout -b %s %s/%s\nThen run gx sync again.", cfg.DefaultBranch, cfg.Remote, cfg.DefaultBranch),
			}
		}
		run.State(fmt.Sprintf("Fast-forwarding %s...", cfg.DefaultBranch))
		if err := git.Checkout(run, cfg.DefaultBranch); err != nil {
			return settleRestore, &cli.Error{
				Message: fmt.Sprintf("Failed to checkout %s.", cfg.DefaultBranch),
				Hint:    "Check git status and switch back manually.",
			}
		}

		ffErr := git.FastForward(run, cfg.Remote, cfg.DefaultBranch)

		// Always attempt to restore the original branch
		if checkoutErr := git.Checkout(run, current); checkoutErr != nil {
			if ffErr != nil {
				return settleRestore, &cli.Error{
					Message: fmt.Sprintf("Failed to fast-forward %s.", cfg.DefaultBranch),
					Hint:    "Your local base branch has diverged. Inspect it manually.",
				}
			}
			return settleRestore, &cli.Error{
				Message: fmt.Sprintf("Failed to checkout %s.", current),
				Hint:    "Check git status and switch back manually.",
			}
		}

		if ffErr != nil {
			if !git.RemoteBranchExists(run, cfg.Remote, cfg.DefaultBranch) {
				return settleRestore, &cli.Error{
					Message: fmt.Sprintf("Remote branch '%s/%s' not found.", cfg.Remote, cfg.DefaultBranch),
					Hint:    "Has it been deleted? Check the remote repository.",
				}
			}
			return settleRestore, &cli.Error{
				Message: fmt.Sprintf("Failed to fast-forward %s.", cfg.DefaultBranch),
				Hint:    "Your local base branch has diverged. Inspect it manually.",
			}
		}

		return integrate(cfg.DefaultBranch, cfg.DefaultBranch)
	})
	run.StopSpinner()
	if err != nil {
		return err
	}

	// 9. Informational hint to push if on feature branch and commits are ready to ship
	if !isProtectedBranch(cfg, current) {
		div, divErr := git.CheckDivergence(run, cfg.Remote, current)
		if divErr == nil && (!div.Tracking || div.Ahead > 0) {
			run.Info("hint: Run gx ship to push.")
		}
	}

	return nil
}

// autoContinueSync continues an in-progress rebase. When a gx stash is present
// (from a previous gx sync run), the resumed session commits it after the
// rebase completes. Only called for gx-orchestrated rebases (Q4).
func autoContinueSync(run runner.Executor, sessionGit syncGit, cfg *config.Config) error {
	if info := git.CurrentRebasePatchInfo(run); info != "" {
		run.Info(fmt.Sprintf("Continuing rebase — applying %s...", info))
	} else {
		run.Info("Continuing rebase...")
	}

	if err := git.RebaseContinue(run); err != nil {
		if git.IsRebaseInProgress(run) {
			files, filesErr := git.ConflictedFiles(run)
			if filesErr != nil {
				return &cli.Error{
					Message: "There are still unresolved conflicts, but the conflicted files could not be listed.",
					Hint:    "Resolve the remaining conflicts, stage the files, then run gx sync --continue again.",
				}
			}
			if len(files) > 0 {
				// Still unresolved conflicts
				msg := "There are still unresolved conflicts."
				if info := git.CurrentRebasePatchInfo(run); info != "" {
					msg = fmt.Sprintf("There are still unresolved conflicts while applying commit %s.", info)
				}
				msg += fmt.Sprintf(" Conflicted files: %s", strings.Join(files, ", "))
				return &cli.Error{
					Message: msg,
					Hint:    "Resolve the remaining conflicts, stage the files, then run gx sync --continue again.",
				}
			}
			// Issue 09b: No conflicts but rebase still paused → empty commit
			msg := "The current commit is empty after conflict resolution."
			if info := git.CurrentRebasePatchInfo(run); info != "" {
				msg = fmt.Sprintf("Commit %s is empty after conflict resolution.", info)
			}
			return &cli.Error{
				Message: msg,
				Hint:    "Run gx sync --skip to skip it, or gx sync --abort to cancel.",
			}
		}
		return &cli.Error{
			Message: "Rebase continue failed.",
		}
	}

	// Rebase succeeded — commit the resumed session (pop + orphan cleanup)
	if err := resume(sessionGit).settle(settleCommit); err != nil {
		return err
	}

	current, err := git.CurrentBranch(run)
	if err == nil && !isProtectedBranch(cfg, current) {
		div, divErr := git.CheckDivergence(run, cfg.Remote, current)
		if divErr == nil && (!div.Tracking || div.Ahead > 0) {
			run.Info("hint: Run gx ship to push.")
		}
	}
	return nil
}

// autoContinueMergeSync continues a paused merge. Behaves like autoContinueSync
// but for merges: runs git merge --continue, then commits the resumed session.
func autoContinueMergeSync(run runner.Executor, sessionGit syncGit, cfg *config.Config) error {
	run.Info("Continuing merge...")

	if err := git.MergeContinue(run); err != nil {
		if git.IsMergeInProgress(run) {
			// Still unresolved conflicts
			files, filesErr := git.ConflictedFiles(run)
			if filesErr != nil {
				return &cli.Error{
					Message: "There are still unresolved conflicts, but the conflicted files could not be listed.",
					Hint:    "Resolve the remaining conflicts, stage the files, then run gx sync --continue again.",
				}
			}
			msg := "There are still unresolved conflicts."
			if len(files) > 0 {
				msg += fmt.Sprintf(" Conflicted files: %s", strings.Join(files, ", "))
			}
			return &cli.Error{
				Message: msg,
				Hint:    "Resolve the remaining conflicts, stage the files, then run gx sync --continue again.",
			}
		}
		return &cli.Error{
			Message: "Merge continue failed.",
		}
	}

	// Merge succeeded — commit the resumed session (pop + orphan cleanup)
	if err := resume(sessionGit).settle(settleCommit); err != nil {
		return err
	}

	current, err := git.CurrentBranch(run)
	if err == nil && !isProtectedBranch(cfg, current) {
		div, divErr := git.CheckDivergence(run, cfg.Remote, current)
		if divErr == nil && (!div.Tracking || div.Ahead > 0) {
			run.Info("hint: Run gx ship to push.")
		}
	}
	return nil
}

// abortWithStash aborts an in-progress operation (rebase or merge) via the
// provided abortFn, then commits the resumed session to restore the dirty
// working tree. If the abort fails, returns an error without settling the
// stash.
func abortWithStash(run runner.Executor, sessionGit syncGit, abortFn func(runner.Executor) error) error {
	if err := abortFn(run); err != nil {
		return &cli.Error{
			Message: "Failed to abort the in-progress operation.",
			Hint:    "Check git status and resolve manually.",
		}
	}
	return resume(sessionGit).settle(settleCommit)
}

// autoSkipSync skips the currently-applying commit during a gx-orchestrated
// rebase. If the rebase is still in progress afterwards, the session stays
// paused (stash kept). When the rebase finished, the session commits.
func autoSkipSync(run runner.Executor, sessionGit syncGit, cfg *config.Config) error {
	if info := git.CurrentRebasePatchInfo(run); info != "" {
		run.Info(fmt.Sprintf("Skipping commit %s...", info))
	} else {
		run.Info("Skipping current commit...")
	}

	if err := git.RebaseSkip(run); err != nil {
		if git.IsRebaseInProgress(run) {
			return &cli.Error{
				Message: "There are still unresolved conflicts.",
				Hint:    "Resolve conflicts and run gx sync --continue, or run gx sync --skip again.",
			}
		}
		return &cli.Error{
			Message: "Rebase skip failed.",
		}
	}

	// Check if more commits remain
	if git.IsRebaseInProgress(run) {
		if info := git.CurrentRebasePatchInfo(run); info != "" {
			run.Info(fmt.Sprintf("Rebase continuing — next commit: %s.", info))
		} else {
			run.Info("Rebase continuing.")
		}
		return nil
	}

	// Rebase finished after skip — commit the resumed session
	run.Success("Rebase completed.")
	if err := resume(sessionGit).settle(settleCommit); err != nil {
		return err
	}

	current, err := git.CurrentBranch(run)
	if err == nil && !isProtectedBranch(cfg, current) {
		div, divErr := git.CheckDivergence(run, cfg.Remote, current)
		if divErr == nil && (!div.Tracking || div.Ahead > 0) {
			run.Info("hint: Run gx ship to push.")
		}
	}
	return nil
}

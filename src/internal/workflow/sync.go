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
	Continue bool
	Rebase   bool
	Merge    bool
	Abort    bool
	Skip     bool
}

func Sync(run *runner.Runner, cfg *config.Config, opts SyncOptions) error {
	sessionGit := newProdSyncGit(run)
	verdict := inspect(run)

	// --abort: abort in-progress rebase or merge, settle the gx stash if present.
	// Only works for gx-orchestrated operations (has gx stash). Manual
	// rebases/merges block with a message telling the user to use git directly.
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
			fmt.Println("Aborting rebase...")
			return abortWithStash(run, sessionGit, git.RebaseAbort)

		case q5ManualMerge:
			// Q5: manual merge — gx sync only manages its own operations
			return &cli.Error{
				Message: "A merge is already in progress.",
				Hint:    "gx sync only manages its own sync operations.\nUse git merge --abort directly.",
			}

		case q6GXMerge:
			// Q6: gx-orchestrated merge — abort, then settle
			fmt.Println("Aborting merge...")
			return abortWithStash(run, sessionGit, git.MergeAbort)

		default:
			// Q1 or Q2: nothing to abort
			return &cli.Error{
				Message: "Nothing to abort.",
				Hint:    "Run gx sync to start a new sync.",
			}
		}
	}

	// 1. Determine current branch
	current, err := git.CurrentBranch(run)
	if err != nil {
		return &cli.Error{
			Message: "Failed to determine current branch.",
		}
	}

	// 3. State machine for rebase/merge + stash interactions — classified
	// once by inspect() (the Q-table lives in repostate.go); this switch is
	// the interpretation for gx sync.
	//
	//    +---+------------------+------------------+---------------+-----------------------------------------+
	//    |   | RebaseInProgress | MergeInProgress  | HasGXStash    | Behavior                                |
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
			return autoSkipSync(run, sessionGit)
		}
		if opts.Continue {
			return autoContinueSync(run, sessionGit)
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
			return autoContinueMergeSync(run, sessionGit)
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
		if opts.Continue || opts.Skip {
			// --continue or --skip but nothing paused
			// Issue 09c: check if rebase already completed
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

	// Detached HEAD is not supported (only reaches here in Q1 or Q2)
	if current == "" {
		return &cli.Error{
			Message: "Detached HEAD is not supported.",
			Hint:    "Checkout a branch first.",
		}
	}

	// 4. Acquire the session: stash the dirty working tree (skipped when clean).
	session, err := begin(sessionGit, current)
	if err != nil {
		return err
	}

	// 5–8. Act phase. Every exit settles through the session: an error
	// restores the stash, a conflict pause keeps it, success commits it.
	return session.run(func() (settle, error) {
		if !git.RemoteExists(run, cfg.Remote) {
			return settleRestore, &cli.Error{
				Message: fmt.Sprintf("Remote '%s' not found.", cfg.Remote),
				Hint:    "Run `git remote -v` to list available remotes, then `gx config remote <name>` to update.",
			}
		}
		if err := git.Fetch(run, cfg.Remote); err != nil {
			return settleRestore, &cli.Error{
				Message: "Failed to fetch remote.",
				Hint:    "Check network connection or remote configuration.",
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

		// 6. If already on base branch — fast-forward only (fetch already done above)
		if current == cfg.DefaultBranch {
			if !git.RemoteBranchExists(run, cfg.Remote, cfg.DefaultBranch) {
				return settleRestore, &cli.Error{
					Message: fmt.Sprintf("Remote branch '%s/%s' not found.", cfg.Remote, cfg.DefaultBranch),
					Hint:    "Has it been deleted? Check the remote repository.",
				}
			}
			if err := git.FastForward(run, cfg.Remote, cfg.DefaultBranch); err != nil {
				return settleRestore, &cli.Error{
					Message: fmt.Sprintf("Local branch '%s' has diverged from '%s/%s'.", cfg.DefaultBranch, cfg.Remote, cfg.DefaultBranch),
					Hint:    "The base branch should not have local commits.\nMove them to a feature branch:\n  git checkout -b fix/description\n  git branch -f " + cfg.DefaultBranch + " " + cfg.Remote + "/" + cfg.DefaultBranch + "\nOr rebase and force-push:\n  git rebase " + cfg.Remote + "/" + cfg.DefaultBranch + "\n  git push --force-with-lease",
				}
			}
			return settleCommit, nil
		}

		// 7. Switch to base branch, fast-forward, return to original
		if !git.LocalBranchExists(run, cfg.DefaultBranch) {
			return settleRestore, &cli.Error{
				Message: fmt.Sprintf("Local branch '%s' does not exist.", cfg.DefaultBranch),
				Hint:    fmt.Sprintf("Create it with: git checkout -b %s %s/%s\nThen run gx sync again.", cfg.DefaultBranch, cfg.Remote, cfg.DefaultBranch),
			}
		}
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

		// 8. Rebase or merge
		// Use flag override if provided, otherwise config
		strategy := cfg.SyncStrategy
		switch {
		case opts.Rebase:
			strategy = "rebase"
		case opts.Merge:
			strategy = "merge"
		}
		switch strategy {
		case "merge":
			if err := git.Merge(run, cfg.DefaultBranch); err != nil {
				if git.IsMergeInProgress(run) {
					// Leave merge state, preserve stash — user resolves and re-runs
					msg := "Merge stopped because of conflicts."
					if files := git.ConflictedFiles(run); len(files) > 0 {
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
			if err := git.Rebase(run, cfg.DefaultBranch); err != nil {
				if git.IsRebaseInProgress(run) {
					// Leave the rebase in progress — user resolves and re-runs gx sync.
					// The session settles as pause — the stash stays for --continue.
					msg := "Rebase stopped because of conflicts."
					if info := git.CurrentRebasePatchInfo(run); info != "" {
						msg = fmt.Sprintf("Rebase stopped because of conflicts while applying commit %s.", info)
					}
					if files := git.ConflictedFiles(run); len(files) > 0 {
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
				// Non-conflict rebase failure
				return settleRestore, &cli.Error{
					Message: "Rebase failed.",
				}
			}
		}

		return settleCommit, nil
	})
}

// autoContinueSync continues an in-progress rebase. When a gx stash is present
// (from a previous gx sync run), the resumed session commits it after the
// rebase completes. Only called for gx-orchestrated rebases (Q4).
func autoContinueSync(run *runner.Runner, sessionGit syncGit) error {
	if info := git.CurrentRebasePatchInfo(run); info != "" {
		fmt.Printf("Continuing rebase — applying %s...\n", info)
	} else {
		fmt.Println("Continuing rebase...")
	}

	if err := git.RebaseContinue(run); err != nil {
		if git.IsRebaseInProgress(run) {
			files := git.ConflictedFiles(run)
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
	return resume(sessionGit).settle(settleCommit)
}

// autoContinueMergeSync continues a paused merge. Behaves like autoContinueSync
// but for merges: runs git merge --continue, then commits the resumed session.
func autoContinueMergeSync(run *runner.Runner, sessionGit syncGit) error {
	fmt.Println("Continuing merge...")

	if err := git.MergeContinue(run); err != nil {
		if git.IsMergeInProgress(run) {
			// Still unresolved conflicts
			msg := "There are still unresolved conflicts."
			if files := git.ConflictedFiles(run); len(files) > 0 {
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
	return resume(sessionGit).settle(settleCommit)
}

// abortWithStash aborts an in-progress operation (rebase or merge) via the
// provided abortFn, then commits the resumed session to restore the dirty
// working tree. If the abort fails, returns an error without settling the
// stash.
func abortWithStash(run *runner.Runner, sessionGit syncGit, abortFn func(*runner.Runner) error) error {
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
func autoSkipSync(run *runner.Runner, sessionGit syncGit) error {
	if info := git.CurrentRebasePatchInfo(run); info != "" {
		fmt.Printf("Skipping commit %s...\n", info)
	} else {
		fmt.Println("Skipping current commit...")
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
			fmt.Printf("Rebase continuing — next commit: %s.\n", info)
		} else {
			fmt.Println("Rebase continuing.")
		}
		return nil
	}

	// Rebase finished after skip — commit the resumed session
	fmt.Println("Rebase completed.")
	return resume(sessionGit).settle(settleCommit)
}

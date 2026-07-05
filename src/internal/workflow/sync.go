package workflow

import (
	"fmt"
	"strings"
	"time"

	"github.com/fathss/gx/internal/cli"
	"github.com/fathss/gx/internal/config"
	"github.com/fathss/gx/internal/git"
	"github.com/fathss/gx/internal/runner"
)

func Sync(run *runner.Runner, cfg *config.Config, argsProvided bool, continueMode bool, flagRebase bool, flagMerge bool, abortMode bool, skipMode bool) error {
	// --abort: abort in-progress rebase or merge, pop gx stash if present.
	// Only works for gx-orchestrated operations (has gx stash). Manual
	// rebases/merges block with a message telling the user to use git directly.
	if abortMode {
		rebaseInProgress := git.IsRebaseInProgress(run)
		mergeInProgress := git.IsMergeInProgress(run)
		hasGXStash := git.HasGXStash(run)

		switch {
		case rebaseInProgress && !hasGXStash:
			// Q3: manual rebase — gx sync only manages its own operations
			return &cli.Error{
				Message: "A rebase is already in progress.",
				Hint:    "gx sync only manages its own sync operations.\nUse git rebase --abort directly.",
			}

		case rebaseInProgress && hasGXStash:
			// Q4: gx-orchestrated rebase — abort, then pop stash
			fmt.Println("Aborting rebase...")
			return abortWithStashPop(run, git.RebaseAbort)

		case mergeInProgress && !hasGXStash:
			// Q5: manual merge — gx sync only manages its own operations
			return &cli.Error{
				Message: "A merge is already in progress.",
				Hint:    "gx sync only manages its own sync operations.\nUse git merge --abort directly.",
			}

		case mergeInProgress && hasGXStash:
			// Q6: gx-orchestrated merge — abort, then pop stash
			fmt.Println("Aborting merge...")
			return abortWithStashPop(run, git.MergeAbort)

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

	// 3. State machine for rebase/merge + stash interactions.
	//
	//    +---+------------------+------------------+---------------+-----------------------------------------+
	//    |   | RebaseInProgress | MergeInProgress  | HasGXStash    | Behavior                                |
	//    +---+------------------+------------------+---------------+-----------------------------------------+
	//    | 1 | false            | false            | false         | normal flow                             |
	//    | 2 | false            | false            | true          | warn orphaned, proceed                  |
	//    | 3 | true             | false            | false         | block: manual rebase (--continue denied) |
	//    | 4 | true             | false            | true          | block: gx rebase (--continue allowed)    |
	//    | 5 | false            | true             | false         | block: manual merge (--continue denied)  |
	//    | 6 | false            | true             | true          | block: gx merge (--continue allowed)     |
	//    +---+------------------+------------------+---------------+-----------------------------------------+
	rebaseInProgress := git.IsRebaseInProgress(run)
	mergeInProgress := git.IsMergeInProgress(run)
	hasGXStash := git.HasGXStash(run)

	switch {
	case rebaseInProgress && hasGXStash:
		// Q4: gx-orchestrated rebase — --continue resumes, --skip skips, plain blocks
		if skipMode {
			return autoSkipSync(run)
		}
		if continueMode {
			return autoContinueSync(run)
		}
		return &cli.Error{
			Message: "A rebase is already in progress.",
			Hint:    "Run gx sync --continue to resume, gx sync --skip to skip, or gx sync --abort to cancel.",
		}

	case mergeInProgress && hasGXStash:
		// Q6: gx-orchestrated merge — --continue resumes, plain blocks
		if skipMode {
			return &cli.Error{
				Message: "--skip is not valid during a merge.",
				Hint:    "Use git merge --continue or --abort directly.",
			}
		}
		if continueMode {
			return autoContinueMergeSync(run)
		}
		return &cli.Error{
			Message: "A merge is already in progress.",
			Hint:    "Run gx sync --continue to resume, or gx sync --abort to cancel.",
		}

	case rebaseInProgress && !hasGXStash:
		// Q3: manual rebase — gx sync only manages its own operations
		if skipMode {
			return &cli.Error{
				Message: "A rebase is already in progress.",
				Hint:    "gx sync only manages its own sync operations.\nUse git rebase --skip directly.",
			}
		}
		return &cli.Error{
			Message: "A rebase is already in progress.",
			Hint:    "gx sync only manages its own sync operations.\nUse git rebase --continue, --skip, or --abort directly.",
		}

	case mergeInProgress && !hasGXStash:
		// Q5: manual merge — gx sync only manages its own operations
		return &cli.Error{
			Message: "A merge is already in progress.",
			Hint:    "gx sync only manages its own sync operations.\nUse git merge --continue or --abort directly.",
		}

	case !rebaseInProgress && !mergeInProgress && (continueMode || skipMode):
		// --continue or --skip but nothing paused
		// Issue 09c: check if rebase already completed
		return &cli.Error{
			Message: "Nothing to continue. Rebase appears to have completed already.",
			Hint:    "Run git status to verify the state, then gx sync to start a fresh sync.",
		}

	case !rebaseInProgress && !mergeInProgress && hasGXStash:
		// Q2: orphaned gx stash — restore before proceeding
		fmt.Println("Restoring stashed changes from previous sync...")
		if err := git.StashPop(run); err != nil {
			return &cli.Error{
				Message: "Failed to restore stashed changes.",
				Hint:    "Run git stash list to inspect stashes, then git stash pop manually.",
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

	// 4. Stash dirty working tree before touching branches.
	//    Skips stash entirely when working tree is clean.
	needsPop := !git.IsClean(run)
	if needsPop {
		label := fmt.Sprintf("%s%s/%d", git.SyncStashPrefix, current, time.Now().Unix())
		fmt.Println("Stashing local changes...")
		if err := git.StashPush(run, label); err != nil {
			return &cli.Error{
				Message: "Failed to stash local changes.",
				Hint:    "Commit or discard your changes manually.",
			}
		}
	}

	// Best-effort restore on failure paths: pop stash to recover original state.
	restoreStash := func() {
		if needsPop {
			if err := git.StashPop(run); err != nil {
				run.Warnf("Failed to restore stashed changes — your changes remain in git stash.", fmt.Sprintf("Error: %v", err))
			}
		}
	}

	// 5. Validate remote exists, then fetch
	if !git.RemoteExists(run, cfg.Remote) {
		restoreStash()
		return &cli.Error{
			Message: fmt.Sprintf("Remote '%s' not found.", cfg.Remote),
			Hint:    "Run `git remote -v` to list available remotes, then `gx config remote <name>` to update.",
		}
	}
	if err := git.Fetch(run, cfg.Remote); err != nil {
		restoreStash()
		return &cli.Error{
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
			restoreStash()
			return &cli.Error{
				Message: fmt.Sprintf("Remote branch '%s/%s' not found.", cfg.Remote, cfg.DefaultBranch),
				Hint:    "Has it been deleted? Check the remote repository.",
			}
		}
		if err := git.FastForward(run, cfg.Remote, cfg.DefaultBranch); err != nil {
			restoreStash()
			return &cli.Error{
				Message: fmt.Sprintf("Local branch '%s' has diverged from '%s/%s'.", cfg.DefaultBranch, cfg.Remote, cfg.DefaultBranch),
				Hint:    "The base branch should not have local commits.\nMove them to a feature branch:\n  git checkout -b fix/description\n  git branch -f " + cfg.DefaultBranch + " " + cfg.Remote + "/" + cfg.DefaultBranch + "\nOr rebase and force-push:\n  git rebase " + cfg.Remote + "/" + cfg.DefaultBranch + "\n  git push --force-with-lease",
			}
		}
		if needsPop {
			fmt.Println("Restoring stashed changes...")
			if err := git.StashPop(run); err != nil {
				return &cli.Error{
					Message: "Stash pop failed — conflicts detected.",
					Hint:    "Resolve the conflicts above, then run: git stash drop",
				}
			}
		}
		// Issue 07: Clean up any orphaned gx-sync stashes after successful sync
		if err := git.DropGXStash(run); err != nil {
			run.Warnf("Failed to clean up gx stash entries.", fmt.Sprintf("Error: %v", err))
		}
		return nil
	}

	// 7. Switch to base branch, fast-forward, return to original
	if !git.LocalBranchExists(run, cfg.DefaultBranch) {
		restoreStash()
		return &cli.Error{
			Message: fmt.Sprintf("Local branch '%s' does not exist.", cfg.DefaultBranch),
			Hint:    fmt.Sprintf("Create it with: git checkout -b %s %s/%s\nThen run gx sync again.", cfg.DefaultBranch, cfg.Remote, cfg.DefaultBranch),
		}
	}
	if err := git.Checkout(run, cfg.DefaultBranch); err != nil {
		restoreStash()
		return &cli.Error{
			Message: fmt.Sprintf("Failed to checkout %s.", cfg.DefaultBranch),
			Hint:    "Check git status and switch back manually.",
		}
	}

	ffErr := git.FastForward(run, cfg.Remote, cfg.DefaultBranch)

	// Always attempt to restore the original branch
	if checkoutErr := git.Checkout(run, current); checkoutErr != nil {
		if ffErr != nil {
			restoreStash()
			return &cli.Error{
				Message: fmt.Sprintf("Failed to fast-forward %s.", cfg.DefaultBranch),
				Hint:    "Your local base branch has diverged. Inspect it manually.",
			}
		}
		restoreStash()
		return &cli.Error{
			Message: fmt.Sprintf("Failed to checkout %s.", current),
			Hint:    "Check git status and switch back manually.",
		}
	}

	if ffErr != nil {
		restoreStash()
		if !git.RemoteBranchExists(run, cfg.Remote, cfg.DefaultBranch) {
			return &cli.Error{
				Message: fmt.Sprintf("Remote branch '%s/%s' not found.", cfg.Remote, cfg.DefaultBranch),
				Hint:    "Has it been deleted? Check the remote repository.",
			}
		}
		return &cli.Error{
			Message: fmt.Sprintf("Failed to fast-forward %s.", cfg.DefaultBranch),
			Hint:    "Your local base branch has diverged. Inspect it manually.",
		}
	}

	// 8. Rebase or merge
	// Use flag override if provided, otherwise config
	strategy := cfg.SyncStrategy
	switch {
	case flagRebase:
		strategy = "rebase"
	case flagMerge:
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
				if needsPop {
					hint += " Your changes were stashed and will be restored automatically."
				}
				return &cli.Error{
					Message: msg,
					Hint:    hint,
				}
			}
			restoreStash()
			return &cli.Error{
				Message: "Merge failed.",
			}
		}
	default:
		if err := git.Rebase(run, cfg.DefaultBranch); err != nil {
			if git.IsRebaseInProgress(run) {
				// Leave the rebase in progress — user resolves and re-runs gx sync.
				// Do NOT restore stash — it stays so gx sync --continue can pop it.
				msg := "Rebase stopped because of conflicts."
				if info := git.CurrentRebasePatchInfo(run); info != "" {
					msg = fmt.Sprintf("Rebase stopped because of conflicts while applying commit %s.", info)
				}
				if files := git.ConflictedFiles(run); len(files) > 0 {
					msg += fmt.Sprintf(" Conflicted files: %s", strings.Join(files, ", "))
				}
				hint := "Resolve the conflicts, stage the files, then run gx sync --continue."
				if needsPop {
					hint += " Your changes were stashed and will be restored automatically."
				}
				return &cli.Error{
					Message: msg,
					Hint:    hint,
				}
			}
			// Non-conflict rebase failure
			restoreStash()
			return &cli.Error{
				Message: "Rebase failed.",
			}
		}
	}

	// 9. Pop stash (only reached on success)
	if needsPop {
		fmt.Println("Restoring stashed changes...")
		if err := git.StashPop(run); err != nil {
			return &cli.Error{
				Message: "Stash pop failed — conflicts detected.",
				Hint:    "Resolve the conflicts above, then run: git stash drop",
			}
		}
	}

	// Issue 07: Clean up any orphaned gx-sync stashes after successful sync
	if err := git.DropGXStash(run); err != nil {
		run.Warnf("Failed to clean up gx stash entries.", fmt.Sprintf("Error: %v", err))
	}

	return nil
}

// autoContinueSync continues an in-progress rebase. When a gx stash is present
// (from a previous gx sync run), it pops the stash after the rebase completes.
// Only called for gx-orchestrated rebases (Q4).
func autoContinueSync(run *runner.Runner) error {
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

	// Rebase succeeded — pop the gx stash if one was left by a previous gx sync
	if git.HasGXStash(run) {
		fmt.Println("Restoring stashed changes...")
		if err := git.StashPop(run); err != nil {
			return &cli.Error{
				Message: "Stash pop failed — conflicts detected.",
				Hint:    "Resolve the conflicts above, then run: git stash drop",
			}
		}
	}

	return nil
}

// autoContinueMergeSync continues a paused merge. Behaves like autoContinueSync
// but for merges: runs git merge --continue, then pops the gx stash if present.
func autoContinueMergeSync(run *runner.Runner) error {
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

	// Merge succeeded — pop the gx stash if one was left by a previous gx sync
	if git.HasGXStash(run) {
		fmt.Println("Restoring stashed changes...")
		if err := git.StashPop(run); err != nil {
			return &cli.Error{
				Message: "Stash pop failed — conflicts detected.",
				Hint:    "Resolve the conflicts above, then run: git stash drop",
			}
		}
	}

	return nil
}

// abortWithStashPop aborts an in-progress operation (rebase or merge) via the
// provided abortFn, then pops the gx stash to restore the dirty working tree.
// If the abort fails, returns an error without attempting to pop the stash.
func abortWithStashPop(run *runner.Runner, abortFn func(*runner.Runner) error) error {
	if err := abortFn(run); err != nil {
		return &cli.Error{
			Message: "Failed to abort the in-progress operation.",
			Hint:    "Check git status and resolve manually.",
		}
	}
	fmt.Println("Restoring stashed changes...")
	if err := git.StashPop(run); err != nil {
		return &cli.Error{
			Message: "Stash pop failed — conflicts detected.",
			Hint:    "Resolve the conflicts above, then run: git stash drop",
		}
	}
	return nil
}

// autoSkipSync skips the currently-applying commit during a gx-orchestrated
// rebase. After the skip, if the rebase is still in progress, it informs the
// user. If the rebase finished, it pops the gx stash.
func autoSkipSync(run *runner.Runner) error {
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

	// Rebase finished after skip — pop stash
	fmt.Println("Rebase completed.")
	if git.HasGXStash(run) {
		fmt.Println("Restoring stashed changes...")
		if err := git.StashPop(run); err != nil {
			return &cli.Error{
				Message: "Stash pop failed — conflicts detected.",
				Hint:    "Resolve the conflicts above, then run: git stash drop",
			}
		}
	}
	return nil
}


package workflow

import (
	"fmt"
	"strings"

	"github.com/fathss/gx/internal/cli"
	"github.com/fathss/gx/internal/config"
	"github.com/fathss/gx/internal/git"
	"github.com/fathss/gx/internal/pager"
	"github.com/fathss/gx/internal/runner"
)

// Clean prunes branches merged into the base branch.
// It always fetches first with --prune. Plain clean only prunes local branches
// from this clone. With remote==true it identifies both local and remote merged
// branches and prompts to prune them in a single unified prompt.
func Clean(run runner.Executor, cfg *config.Config, remote, yes bool) error {
	// --yes is prompter vocabulary: seat it once for confirmation.
	prompts.autoYes = yes

	// 1. Fetch from remote with --prune so merged decision reflects current remote state,
	// and stale remote-tracking refs whose remote branches were already deleted are pruned.
	run.State(fmt.Sprintf("Fetching %s...", cfg.Remote))
	if err := git.Fetch(run, cfg.Remote); err != nil {
		return &cli.Error{
			Message: fmt.Sprintf("Failed to fetch from %q.", cfg.Remote),
			Hint:    "Check the error above; verify the remote with `git remote -v`.",
		}
	}

	current, _ := git.CurrentBranch(run)

	// Branches checked out in any worktree cannot be deleted with `git
	// branch -d`. Exclude them explicitly instead of relying on git's "+ "
	// listing marker being dropped by the parser.
	worktrees, err := git.WorktreeBranches(run)
	if err != nil {
		return &cli.Error{
			Message: "Failed to list git worktrees.",
			Hint:    "Check that `git worktree list` works in this repository.",
		}
	}

	// 2. Local candidates: merged into local defaultBranch.
	localCandidates, err := git.MergedBranches(run, cfg.DefaultBranch)
	if err != nil {
		return &cli.Error{
			Message: fmt.Sprintf("Failed to list merged local branches against %q.", cfg.DefaultBranch),
			Hint:    "Check that the configured defaultBranch exists locally.",
		}
	}
	filteredLocal := filterLocalBranches(localCandidates, cfg, current, worktrees)
	warnSkippedWorktrees(run, localCandidates, cfg, current, worktrees)

	// Check that the remote base branch exists.
	if !git.RemoteBranchExists(run, cfg.Remote, cfg.DefaultBranch) {
		return &cli.Error{
			Message: fmt.Sprintf("Failed to list merged remote-tracking refs against %q/%q.", cfg.Remote, cfg.DefaultBranch),
			Hint:    "Check that the configured remote and defaultBranch exist.",
		}
	}

	// 3. Remote candidates (only when --remote requested): merged into <remote>/<defaultBranch>.
	var remoteToDelete []string
	if remote {
		remoteCandidates, err := git.MergedRemoteBranches(run, cfg.Remote, cfg.DefaultBranch)
		if err != nil {
			return &cli.Error{
				Message: fmt.Sprintf("Failed to list merged remote-tracking refs against %q/%q.", cfg.Remote, cfg.DefaultBranch),
				Hint:    "Check that the configured remote and defaultBranch exist.",
			}
		}
		remoteToDelete = filterBranches(remoteCandidates, cfg)
	}

	totalCandidates := len(filteredLocal) + len(remoteToDelete)
	if totalCandidates == 0 {
		run.StopSpinner()
		run.Success("Pruned 0 branches.")
		return nil
	}

	// 4. Confirmation prompt.
	run.StopSpinner()
	if !yes {
		var b strings.Builder
		if remote {
			b.WriteString("Merged branches eligible for pruning:")
			if len(filteredLocal) > 0 {
				b.WriteString("\n\nLocal branches:")
				for _, br := range filteredLocal {
					b.WriteString(fmt.Sprintf("\n  %s", br))
				}
			}
			if len(remoteToDelete) > 0 {
				b.WriteString(fmt.Sprintf("\n\nRemote branches on %s:", cfg.Remote))
				for _, br := range remoteToDelete {
					b.WriteString(fmt.Sprintf("\n  %s", git.Qualify(cfg.Remote, br)))
				}
			}
		} else {
			b.WriteString("Merged branches eligible for pruning in this clone:")
			b.WriteString("\n\nLocal branches:")
			for _, br := range filteredLocal {
				b.WriteString(fmt.Sprintf("\n  %s", br))
			}
		}
		_ = pager.Print(b.String())
	}

	var promptText string
	switch {
	case len(filteredLocal) > 0 && len(remoteToDelete) > 0:
		promptText = fmt.Sprintf("Prune %d local branch(es) and delete %d remote branch(es) on %s?", len(filteredLocal), len(remoteToDelete), cfg.Remote)
	case len(filteredLocal) > 0:
		promptText = fmt.Sprintf("Prune %d merged branch(es) from this clone?", len(filteredLocal))
	default:
		promptText = fmt.Sprintf("Delete %d remote branch(es) on %s?", len(remoteToDelete), cfg.Remote)
	}

	confirmed, err := prompts.Confirm(promptText)
	if err != nil {
		return err
	}
	if !confirmed {
		run.Success("Pruned 0 branches.")
		return nil
	}

	// 5. Delete in order: local, remote.
	run.State(fmt.Sprintf("Pruning %d branches...", totalCandidates))
	prunedLocal := 0
	prunedRemote := 0
	var firstErr error

	for _, br := range filteredLocal {
		if err := git.DeleteLocalBranch(run, br); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		prunedLocal++
	}

	if remote {
		for _, br := range remoteToDelete {
			if err := git.DeleteRemoteBranch(run, cfg.Remote, br); err != nil {
				// Another collaborator may have deleted the branch between our
				// fetch and our push. The desired end state is already true.
				if git.IsRemoteRefMissing(err) {
					prunedRemote++
					continue
				}
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			prunedRemote++
		}
	}

	// 6. Summary line.
	totalPruned := prunedLocal + prunedRemote
	if firstErr != nil {
		run.Info(fmt.Sprintf("Pruned %d branches — their commits remain on %s.", totalPruned, cfg.DefaultBranch))
		return &cli.Error{
			Message: "Failed to prune some branches.",
			Hint:    fmt.Sprintf("Check the error above and try again. Pruned branches' commits remain on %s.", cfg.DefaultBranch),
		}
	}

	if remote && prunedLocal > 0 && prunedRemote > 0 {
		run.Success(fmt.Sprintf("Pruned %d local and %d remote branch(es).", prunedLocal, prunedRemote))
	} else {
		run.Success(fmt.Sprintf("Pruned %d branches.", totalPruned))
	}
	return nil
}

// filterBranches drops the default branch and protected branches from bare
// branch names. Both local and remote-tracking candidates arrive already
// normalized by the git layer, so policy is the only concern here.
func filterBranches(candidates []string, cfg *config.Config) []string {
	protectedSet := make(map[string]bool, len(cfg.ProtectedBranches))
	for _, p := range cfg.ProtectedBranches {
		protectedSet[p] = true
	}

	var out []string
	for _, name := range candidates {
		if name == "" || name == cfg.DefaultBranch || protectedSet[name] {
			continue
		}
		out = append(out, name)
	}
	return out
}

// filterLocalBranches applies filterBranches and additionally drops branches
// that cannot be deleted locally: the current branch and any branch checked
// out in a worktree. Remote-tracking candidates deliberately skip both checks:
// origin/<branch> stays prunable while its local counterpart is in use.
func filterLocalBranches(candidates []string, cfg *config.Config, current string, worktrees map[string]bool) []string {
	var out []string
	for _, name := range filterBranches(candidates, cfg) {
		if name == current || worktrees[name] {
			continue
		}
		out = append(out, name)
	}
	return out
}

// warnSkippedWorktrees tells the user why otherwise-prunable branches were
// left alone. Only branches that would have been candidates are reported, so
// the warning does not fire for the current branch or protected branches.
func warnSkippedWorktrees(run runner.Executor, candidates []string, cfg *config.Config, current string, worktrees map[string]bool) {
	var skipped []string
	for _, name := range filterBranches(candidates, cfg) {
		if name != current && worktrees[name] {
			skipped = append(skipped, name)
		}
	}
	if len(skipped) == 0 {
		return
	}
	run.Warnf(
		fmt.Sprintf("Skipping merged branches checked out in other worktrees: %s.", strings.Join(skipped, ", ")),
		"Remove the worktree or delete the branch manually with git branch -d.",
	)
}

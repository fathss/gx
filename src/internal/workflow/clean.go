package workflow

import (
	"fmt"
	"strings"

	"github.com/fathss/gx/internal/cli"
	"github.com/fathss/gx/internal/config"
	"github.com/fathss/gx/internal/git"
	"github.com/fathss/gx/internal/runner"
)

// Clean prunes branches merged into the base branch.
// It always fetches first, then prompts before deleting local branches and
// remote-tracking refs merged into the relevant base. With remote==true it
// prompts a second time before deleting remote branches. With yes==true both
// prompts are skipped and every candidate is deleted.
func Clean(run runner.Executor, cfg *config.Config, remote, yes bool) error {
	// --yes is prompter vocabulary: seat it once for both confirmations.
	prompts.autoYes = yes

	// 1. Fetch from remote so merged decision reflects current remote state.
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

	// 3. Remote-tracking candidates: merged into <remote>/<defaultBranch>.
	remoteCandidates, err := git.MergedRemoteBranches(run, cfg.Remote, cfg.DefaultBranch)
	if err != nil {
		return &cli.Error{
			Message: fmt.Sprintf("Failed to list merged remote-tracking refs against %q/%q.", cfg.Remote, cfg.DefaultBranch),
			Hint:    "Check that the configured remote and defaultBranch exist.",
		}
	}
	filteredRemoteTracking := filterBranches(remoteCandidates, cfg)

	// 4. Remote deletion candidates mirror the remote-tracking list; both are
	//    bare branch names below the seam.
	remoteToDelete := filteredRemoteTracking

	// 5. Confirmations, both collected before any mutation.
	//    Local scope first (local branches + remote-tracking refs), then the
	//    remote scope when --remote was requested. Every prompt defaults to No,
	//    and --yes answers both with yes without asking.
	var doLocalDelete bool
	if len(filteredLocal) > 0 || len(filteredRemoteTracking) > 0 {
		var b strings.Builder
		b.WriteString(fmt.Sprintf("Prune %d merged branch(es) from this clone?", len(filteredLocal)+len(filteredRemoteTracking)))
		for _, br := range filteredLocal {
			b.WriteString(fmt.Sprintf("\n  %s", br))
		}
		for _, br := range filteredRemoteTracking {
			b.WriteString(fmt.Sprintf("\n  %s", git.Qualify(cfg.Remote, br)))
		}
		confirmed, err := prompts.Confirm(b.String())
		if err != nil {
			return err
		}
		doLocalDelete = confirmed
	}

	var doRemoteDelete bool
	if remote && len(remoteToDelete) > 0 {
		var b strings.Builder
		b.WriteString(fmt.Sprintf("Delete %d remote branch(es) on %s?", len(remoteToDelete), cfg.Remote))
		for _, br := range remoteToDelete {
			b.WriteString(fmt.Sprintf("\n  %s", git.Qualify(cfg.Remote, br)))
		}
		confirmed, err := prompts.Confirm(b.String())
		if err != nil {
			return err
		}
		doRemoteDelete = confirmed
	}

	// 6. Delete in order: local, remote-tracking, remote.
	pruned := 0
	var firstErr error

	if doLocalDelete {
		for _, br := range filteredLocal {
			if err := git.DeleteLocalBranch(run, br); err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			pruned++
		}

		for _, br := range filteredRemoteTracking {
			if err := git.DeleteRemoteTrackingBranch(run, cfg.Remote, br); err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			pruned++
		}
	}

	if doRemoteDelete {
		for _, br := range remoteToDelete {
			if err := git.DeleteRemoteBranch(run, cfg.Remote, br); err != nil {
				// Another collaborator may have deleted the branch between our
				// fetch and our push. The desired end state is already true.
				if git.IsRemoteRefMissing(err) {
					pruned++
					continue
				}
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			pruned++
		}
	}

	// 7. Summary line.
	if firstErr != nil {
		fmt.Printf("Pruned %d branches — their commits remain on %s.\n", pruned, cfg.DefaultBranch)
		return &cli.Error{
			Message: "Failed to prune some branches.",
			Hint:    fmt.Sprintf("Check the error above and try again. Pruned branches' commits remain on %s.", cfg.DefaultBranch),
		}
	}
	fmt.Printf("Pruned %d branches.\n", pruned)
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

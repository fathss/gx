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
// It always fetches first, then deletes local branches and remote-tracking
// refs merged into the relevant base. With remote==true it prompts and
// deletes remote branches as well.
func Clean(run *runner.Runner, cfg *config.Config, remote bool) error {
	// 1. Fetch from remote so merged decision reflects current remote state.
	if err := git.Fetch(run, cfg.Remote); err != nil {
		return &cli.Error{
			Message: fmt.Sprintf("Failed to fetch from %q.", cfg.Remote),
			Hint:    "Check network connection or remote configuration.",
		}
	}

	current, _ := git.CurrentBranch(run)

	// 2. Local candidates: merged into local defaultBranch.
	localCandidates, err := git.MergedBranches(run, cfg.DefaultBranch)
	if err != nil {
		return &cli.Error{
			Message: fmt.Sprintf("Failed to list merged local branches against %q.", cfg.DefaultBranch),
			Hint:    "Check that the configured defaultBranch exists locally.",
		}
	}
	filteredLocal := filterBranches(localCandidates, cfg, current, cfg.Remote, false)

	// 3. Remote-tracking candidates: merged into <remote>/<defaultBranch>.
	remoteCandidates, err := git.MergedRemoteBranches(run, cfg.Remote, cfg.DefaultBranch)
	if err != nil {
		return &cli.Error{
			Message: fmt.Sprintf("Failed to list merged remote-tracking refs against %q/%q.", cfg.Remote, cfg.DefaultBranch),
			Hint:    "Check that the configured remote and defaultBranch exist.",
		}
	}
	filteredRemoteTracking := filterBranches(remoteCandidates, cfg, current, cfg.Remote, true)

	// 4. Remote deletion candidates (branch-name portion of remote-tracking list).
	var remoteToDelete []string
	if remote && len(filteredRemoteTracking) > 0 {
		for _, ref := range filteredRemoteTracking {
			trimmed := strings.TrimSpace(ref)
			// ref is like "origin/feature/foo"
			if !strings.HasPrefix(trimmed, cfg.Remote+"/") {
				continue
			}
			branch := strings.TrimPrefix(trimmed, cfg.Remote+"/")
			if branch == "" || strings.Contains(branch, " ") || strings.Contains(branch, "->") {
				continue
			}
			remoteToDelete = append(remoteToDelete, branch)
		}
	}

	// 5. Confirmation for remote deletion (Option A).
	var doRemoteDelete bool
	if remote && len(remoteToDelete) > 0 {
		var b strings.Builder
		b.WriteString(fmt.Sprintf("Delete %d remote branch(es) on %s?", len(remoteToDelete), cfg.Remote))
		for _, br := range remoteToDelete {
			b.WriteString(fmt.Sprintf("\n  %s/%s", cfg.Remote, br))
		}
		confirmed, err := promptConfirm(b.String())
		if err != nil {
			return err
		}
		doRemoteDelete = confirmed
	}

	// 6. Delete in order: local, remote-tracking, remote.
	pruned := 0
	var firstErr error

	for _, br := range filteredLocal {
		trimmed := strings.TrimSpace(br)
		// MergedBranches already strips "* " but be safe.
		if strings.HasPrefix(trimmed, "* ") {
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "* "))
		}
		if trimmed == "" {
			continue
		}
		if err := git.DeleteLocalBranch(run, trimmed); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		pruned++
	}

	for _, ref := range filteredRemoteTracking {
		trimmed := strings.TrimSpace(ref)
		if !strings.HasPrefix(trimmed, cfg.Remote+"/") {
			continue
		}
		branch := strings.TrimPrefix(trimmed, cfg.Remote+"/")
		if branch == "" || strings.Contains(branch, "->") {
			continue
		}
		if err := git.DeleteRemoteTrackingBranch(run, cfg.Remote, branch); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		pruned++
	}

	if doRemoteDelete {
		for _, br := range remoteToDelete {
			if err := git.DeleteRemoteBranch(run, cfg.Remote, br); err != nil {
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

// filterBranches removes protected, current, default, and HEAD refs from the
// candidate list. For remote-tracking refs it matches the branch-name portion
// (stripping the <remote>/ prefix) so that protectedBranches: ["main"] matches
// "origin/main".
func filterBranches(candidates []string, cfg *config.Config, current, remote string, isRemoteTracking bool) []string {
	protectedSet := make(map[string]bool, len(cfg.ProtectedBranches))
	for _, p := range cfg.ProtectedBranches {
		protectedSet[p] = true
	}

	var out []string
	for _, cand := range candidates {
		trimmed := strings.TrimSpace(cand)
		if trimmed == "" {
			continue
		}
		// Local candidates may have "* " prefix.
		if !isRemoteTracking && strings.HasPrefix(trimmed, "* ") {
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "* "))
		}
		// Remote-tracking refs may contain symbolic-ref arrow: "origin/HEAD -> origin/main"
		if isRemoteTracking && strings.Contains(trimmed, "->") {
			continue
		}

		var branchName string
		if isRemoteTracking {
			if !strings.HasPrefix(trimmed, remote+"/") {
				continue
			}
			branchName = strings.TrimPrefix(trimmed, remote+"/")
			if branchName == "HEAD" {
				continue
			}
			if branchName == cfg.DefaultBranch {
				continue
			}
			if protectedSet[branchName] {
				continue
			}
		} else {
			branchName = trimmed
			if branchName == cfg.DefaultBranch {
				continue
			}
			if protectedSet[branchName] {
				continue
			}
			if current != "" && branchName == current {
				continue
			}
		}
		out = append(out, trimmed)
	}
	return out
}

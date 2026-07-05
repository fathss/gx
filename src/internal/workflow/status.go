package workflow

import (
	"fmt"
	"strings"

	"github.com/fathss/gx/internal/cli"
	"github.com/fathss/gx/internal/config"
	"github.com/fathss/gx/internal/git"
	"github.com/fathss/gx/internal/runner"
)

// Status prints a human-readable snapshot of the repository state.
//
// The six sections are printed in fixed order:
//  1. Branch / HEAD state (with rebase/merge annotation)
//  2. (Conflicted files, if any)
//  3. Staged files (grouped by type)
//  4. Unstaged files (modified tracked + untracked)
//  5. Stash count
//  6. Ahead/behind the upstream
//  7. Recent commits (last 5)
func Status(run *runner.Runner, cfg *config.Config) error {
	// 1. Branch / HEAD state
	headState, err := git.GetHEADState(run)
	if err != nil {
		return &cli.Error{
			Message: "Failed to determine HEAD state.",
		}
	}

	rebaseInProgress := git.IsRebaseInProgress(run)
	mergeInProgress := git.IsMergeInProgress(run)

	var branchLine string
	if headState.IsDetached {
		branchLine = fmt.Sprintf("HEAD detached at %s", headState.ShortHash)
	} else {
		branchLine = fmt.Sprintf("On branch %s", headState.Branch)
	}

	switch {
	case rebaseInProgress:
		branchLine += " (rebase in progress)"
	case mergeInProgress:
		branchLine += " (merge in progress)"
	}

	// 2. Parse status (staged, unstaged, untracked, conflicted)
	ps, err := git.GetParsedStatus(run)
	if err != nil {
		return &cli.Error{
			Message: "Failed to parse repository status.",
		}
	}

	// Build sections
	var sections []string
	sections = append(sections, branchLine)

	// Conflicted section (if any)
	if len(ps.ConflictedFiles) > 0 {
		conflictedLines := []string{"Conflicted:"}
		for _, f := range ps.ConflictedFiles {
			conflictedLines = append(conflictedLines, fmt.Sprintf("  both modified: %s", f))
		}
		sections = append(sections, strings.Join(conflictedLines, "\n"))
	}

	// Staged section (always present)
	stagedLines := []string{"Staged:"}
	if totalStaged(ps.StagedFiles) == 0 {
		stagedLines = append(stagedLines, "  (none)")
	} else {
		// Print in canonical order: M, A, D, R
		for _, letter := range []string{"M", "A", "D", "R"} {
			files, ok := ps.StagedFiles[letter]
			if !ok || len(files) == 0 {
				continue
			}
			label := git.FormatStagedLabel(letter)
			for _, f := range files {
				stagedLines = append(stagedLines, fmt.Sprintf("  %s:  %s", label, f))
			}
		}
	}
	sections = append(sections, strings.Join(stagedLines, "\n"))

	// Unstaged section (only when either unstaged or untracked has content)
	if len(ps.UnstagedFiles) > 0 || len(ps.UntrackedFiles) > 0 {
		unstagedLines := []string{}

		if len(ps.UnstagedFiles) > 0 {
			unstagedLines = append(unstagedLines, "Unstaged:")
			for _, f := range ps.UnstagedFiles {
				unstagedLines = append(unstagedLines, fmt.Sprintf("  modified:  %s", f))
			}
		}

		if len(ps.UntrackedFiles) > 0 {
			if len(unstagedLines) > 0 {
				// Separate sub-groups with a blank line within the section
				unstagedLines = append(unstagedLines, "")
			}
			unstagedLines = append(unstagedLines, "Untracked:")
			for _, f := range ps.UntrackedFiles {
				unstagedLines = append(unstagedLines, fmt.Sprintf("  %s", f))
			}
		}

		sections = append(sections, strings.Join(unstagedLines, "\n"))
	}

	// Stash count section
	stashes, err := git.StashList(run)
	if err != nil {
		return &cli.Error{
			Message: "Failed to list stashes.",
		}
	}
	stashLine := "Stashes:"
	if len(stashes) == 0 {
		stashLine += " (none)"
	} else {
		stashLine += fmt.Sprintf(" %d", len(stashes))
	}
	sections = append(sections, stashLine)

	// Ahead/behind section
	current := headState.Branch
	aheadBehindLine := "Ahead/behind:"

	if current == "" {
		// Detached HEAD — not applicable
		aheadBehindLine += " (detached HEAD)"
	} else if !git.HasUpstream(run, current) {
		aheadBehindLine += " (no upstream configured)"
	} else {
		upstream := upstreamRef(run, current)
		remote, branch := parseUpstreamRef(upstream)

		ahead, behind, err := git.AheadBehind(run, remote, branch)
		if err != nil {
			aheadBehindLine += " (upstream error)"
		} else if ahead == -1 && behind == -1 {
			// Remote-tracking ref doesn't exist yet
			aheadBehindLine += " (no upstream configured)"
		} else if ahead == 0 && behind == 0 {
			aheadBehindLine += fmt.Sprintf(" up to date with %s", upstream)
		} else {
			var parts []string
			if ahead > 0 {
				parts = append(parts, fmt.Sprintf("%d ahead", ahead))
			}
			if behind > 0 {
				parts = append(parts, fmt.Sprintf("%d behind", behind))
			}
			// "N ahead, M behind <upstream>" or "N ahead of <upstream>" or "N behind <upstream>"
			connector := " "
			if ahead > 0 && behind == 0 {
				connector = " of "
			} else if behind > 0 && ahead == 0 {
				connector = " "
			} else {
				connector = " "
			}
			aheadBehindLine += fmt.Sprintf(" %s%s%s", strings.Join(parts, ", "), connector, upstream)
		}
	}
	sections = append(sections, aheadBehindLine)

	// Recent commits section
	commits, err := git.RecentCommits(run, 5)
	if err != nil {
		return &cli.Error{
			Message: "Failed to retrieve recent commits.",
		}
	}
	commitLines := []string{"Recent commits:"}
	if len(commits) == 0 {
		commitLines = append(commitLines, "  (none — no commits yet)")
	} else {
		for _, c := range commits {
			commitLines = append(commitLines, fmt.Sprintf("  %s  (%s)  %s", c.ShortHash, c.RelativeDate, c.Subject))
		}
	}
	sections = append(sections, strings.Join(commitLines, "\n"))

	// Print the assembled output (sections separated by blank lines)
	fmt.Println(strings.Join(sections, "\n\n"))

	return nil
}

// totalStaged returns the total number of staged files across all status types.
func totalStaged(staged map[string][]string) int {
	total := 0
	for _, files := range staged {
		total += len(files)
	}
	return total
}

// upstreamRef returns the full upstream tracking ref for the current branch,
// e.g. "origin/feature/test". Returns "" if no upstream is configured.
func upstreamRef(run *runner.Runner, branch string) string {
	out, err := run.Output("rev-parse", "--abbrev-ref", branch+"@{upstream}")
	if err != nil {
		return ""
	}
	return out
}

// parseUpstreamRef splits an upstream ref like "origin/feature/test"
// into remote ("origin") and branch ("feature/test").
func parseUpstreamRef(ref string) (remote, branch string) {
	idx := strings.Index(ref, "/")
	if idx == -1 {
		return ref, ""
	}
	return ref[:idx], ref[idx+1:]
}

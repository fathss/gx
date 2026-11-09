package workflow

import (
	"fmt"
	"strings"

	"github.com/fathss/gx/internal/cli"
	"github.com/fathss/gx/internal/config"
	"github.com/fathss/gx/internal/git"
	"github.com/fathss/gx/internal/pager"
	"github.com/fathss/gx/internal/runner"
	"github.com/fathss/gx/internal/ui"
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
func Status(run runner.Executor, cfg *config.Config) error {
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

	stagedStats, _ := git.DiffNumstat(run, true)
	unstagedStats, _ := git.DiffNumstat(run, false)

	// Build sections
	var sections []string
	sections = append(sections, branchLine)

	// Conflicted section (if any)
	if len(ps.ConflictedFiles) > 0 {
		conflictedLines := []string{"Conflicted:"}
		for _, f := range ps.ConflictedFiles {
			badge := ui.StatusBadge("C")
			conflictedLines = append(conflictedLines, fmt.Sprintf("  %s  both modified: %s", badge, f))
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
			badge := ui.StatusBadge(letter)
			for _, f := range files {
				statStr := ""
				if stat, ok := stagedStats[f]; ok {
					statStr = git.FormatDiffStat(stat)
				}
				if statStr != "" {
					stagedLines = append(stagedLines, fmt.Sprintf("  %s  %s %s", badge, f, statStr))
				} else {
					stagedLines = append(stagedLines, fmt.Sprintf("  %s  %s", badge, f))
				}
			}
		}
	}
	sections = append(sections, strings.Join(stagedLines, "\n"))

	// Unstaged section (only when either unstaged or untracked has content)
	if len(ps.UnstagedFiles) > 0 || len(ps.UntrackedFiles) > 0 {
		unstagedLines := []string{"Unstaged:"}

		for _, f := range ps.UnstagedFiles {
			letter := "M"
			if st, ok := ps.UnstagedByStatus[f]; ok {
				letter = st
			}
			badge := ui.StatusBadge(letter)
			statStr := ""
			if stat, ok := unstagedStats[f]; ok {
				statStr = git.FormatDiffStat(stat)
			}
			if statStr != "" {
				unstagedLines = append(unstagedLines, fmt.Sprintf("  %s  %s %s", badge, f, statStr))
			} else {
				unstagedLines = append(unstagedLines, fmt.Sprintf("  %s  %s", badge, f))
			}
		}

		for _, f := range ps.UntrackedFiles {
			badge := ui.StatusBadge("U")
			unstagedLines = append(unstagedLines, fmt.Sprintf("  %s  %s", badge, f))
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
	} else {
		// gx's one notion of upstream: the configured remote's copy of the branch.
		upstream := git.Qualify(cfg.Remote, current)
		div, divErr := git.CheckDivergence(run, cfg.Remote, current)
		switch {
		case divErr != nil:
			aheadBehindLine += " (upstream error)"
		case !div.Tracking:
			aheadBehindLine += " (no upstream configured)"
		case div.Ahead == 0 && div.Behind == 0:
			aheadBehindLine += fmt.Sprintf(" up to date with %s", upstream)
		default:
			var parts []string
			if div.Ahead > 0 {
				parts = append(parts, fmt.Sprintf("%d ahead", div.Ahead))
			}
			if div.Behind > 0 {
				parts = append(parts, fmt.Sprintf("%d behind", div.Behind))
			}
			// "N ahead, M behind <upstream>" or "N ahead of <upstream>" or "N behind <upstream>"
			connector := " "
			if div.Ahead > 0 && div.Behind == 0 {
				connector = " of "
			} else if div.Behind > 0 && div.Ahead == 0 {
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
	return pager.Print(strings.Join(sections, "\n\n"))
}

// totalStaged returns the total number of staged files across all status types.
func totalStaged(staged map[string][]string) int {
	total := 0
	for _, files := range staged {
		total += len(files)
	}
	return total
}

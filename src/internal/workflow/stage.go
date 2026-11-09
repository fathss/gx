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

// StageOptions holds parameters for gx stage.
type StageOptions struct {
	Files           []string
	All             bool
	Patch           bool
	AllowSensitive  bool
	ExcludePatterns []string
}

// Stage stages changes into the git index with safety checks.
func Stage(run runner.Executor, cfg *config.Config, opts StageOptions) error {
	// 1. Pre-flight: block if rebase or merge in progress
	if err := requireNoInProgress(inspect(run)); err != nil {
		return err
	}

	var filesToStage []string
	hasSpecificFiles := len(opts.Files) > 0

	if hasSpecificFiles {
		filesToStage = opts.Files
	} else if opts.Patch && len(opts.ExcludePatterns) == 0 {
		filesToStage = nil
	} else {
		// Scan working tree — only inspect files with unstaged changes or untracked files
		ps, err := git.GetParsedStatus(run)
		if err != nil {
			return &cli.Error{
				Message: "Failed to scan working tree.",
			}
		}

		treeUnstaged := ps.UnstagedFiles
		treeUntracked := ps.UntrackedFiles

		// Filter out submodules
		submodules, subErr := git.SubmodulePaths(run)
		if subErr == nil && len(submodules) > 0 {
			var filteredUnstaged, filteredUntracked []string
			for _, f := range treeUnstaged {
				if submodules[f] {
					run.Info(fmt.Sprintf("  [submodule] %s (skipped — use git add manually)", f))
				} else {
					filteredUnstaged = append(filteredUnstaged, f)
				}
			}
			for _, f := range treeUntracked {
				if submodules[f] {
					run.Info(fmt.Sprintf("  [submodule] %s (skipped — use git add manually)", f))
				} else {
					filteredUntracked = append(filteredUntracked, f)
				}
			}
			treeUnstaged = filteredUnstaged
			treeUntracked = filteredUntracked
		}

		if len(treeUnstaged) == 0 && len(treeUntracked) == 0 {
			hasStaged, _ := git.HasStagedChanges(run)
			if hasStaged {
				run.Info("No new changes to stage.")
			} else {
				run.Info("Working tree is clean, nothing to stage.")
			}
			return nil
		}

		if !opts.Patch {
			unstagedStats, _ := git.DiffNumstat(run, false)

			var previewLines []string
			previewLines = append(previewLines, "Unstaged tracked files:")
			if len(treeUnstaged) == 0 {
				previewLines = append(previewLines, "  (none)")
			} else {
				for _, f := range treeUnstaged {
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
						previewLines = append(previewLines, fmt.Sprintf("  %s  %s %s", badge, f, statStr))
					} else {
						previewLines = append(previewLines, fmt.Sprintf("  %s  %s", badge, f))
					}
				}
			}

			previewLines = append(previewLines, "", "New untracked files:")
			if len(treeUntracked) == 0 {
				previewLines = append(previewLines, "  (none)")
			} else {
				for _, f := range treeUntracked {
					badge := ui.StatusBadge("U")
					previewLines = append(previewLines, fmt.Sprintf("  %s  %s", badge, f))
				}
			}

			_ = pager.Print(strings.Join(previewLines, "\n"))

			stageUntracked := opts.All
			if len(treeUntracked) > 0 && !opts.All {
				var err error
				stageUntracked, err = prompts.Confirm(fmt.Sprintf("Stage %d untracked file(s)?", len(treeUntracked)))
				if err != nil {
					return err
				}
			}

			if stageUntracked {
				filesToStage = append(treeUnstaged, treeUntracked...)
			} else {
				filesToStage = treeUnstaged
			}
		} else {
			filesToStage = append(treeUnstaged, treeUntracked...)
		}
	}

	// Apply exclude patterns
	if len(opts.ExcludePatterns) > 0 {
		filesToStage = git.FilterExcluded(filesToStage, opts.ExcludePatterns)
		if hasSpecificFiles {
			opts.Files = git.FilterExcluded(opts.Files, opts.ExcludePatterns)
			if len(opts.Files) == 0 {
				hasSpecificFiles = false
				filesToStage = nil
			}
		}
	}

	// Sensitive pattern check
	if !opts.AllowSensitive {
		var checkFiles []string
		if hasSpecificFiles {
			checkFiles = opts.Files
		} else if filesToStage != nil {
			checkFiles = filesToStage
		} else {
			ps, _ := git.GetParsedStatus(run)
			if ps != nil {
				checkFiles = append(ps.UnstagedFiles, ps.UntrackedFiles...)
			}
		}
		matches := git.MatchSensitivePatterns(checkFiles, cfg.SensitivePatterns)
		if len(matches) > 0 {
			if opts.Patch {
				run.Warnf(
					"Sensitive file(s) detected in the working tree:\n  "+strings.Join(matches, "\n  "),
					"In patch mode you can choose to skip hunks from these files.\nUse --allow-sensitive to suppress this warning.",
				)
			} else {
				msg := "Sensitive file(s) detected — refusing to stage:\n"
				for _, m := range matches {
					msg += fmt.Sprintf("  %s\n", m)
				}
				return &cli.Error{
					Message: strings.TrimRight(msg, "\n"),
					Hint:    "Remove or gitignore the files, or use --allow-sensitive to override.",
				}
			}
		}
	}

	// Record staged state before execution so summary only reports files staged in this run
	stagedBefore, _ := git.DiffNumstat(run, true)
	if stagedBefore == nil {
		stagedBefore = make(map[string]git.FileDiffStat)
	}

	// Stage execution
	if opts.Patch {
		run.StopSpinner()
		if len(opts.ExcludePatterns) > 0 {
			if len(filesToStage) > 0 {
				if err := git.AddPatch(run, filesToStage...); err != nil {
					return &cli.Error{
						Message: "Patch staging failed.",
						Hint:    "Check the error above and try again.",
					}
				}
			}
		} else {
			if err := git.AddPatch(run, opts.Files...); err != nil {
				return &cli.Error{
					Message: "Patch staging failed.",
					Hint:    "Check the error above and try again.",
				}
			}
		}
	} else if hasSpecificFiles {
		run.State("Staging changes...")
		if err := git.Add(run, opts.Files...); err != nil {
			run.StopSpinner()
			return &cli.Error{
				Message: "Failed to stage files.",
			}
		}
	} else if len(filesToStage) > 0 {
		run.State("Staging changes...")
		if err := git.Add(run, filesToStage...); err != nil {
			run.StopSpinner()
			return &cli.Error{
				Message: "Failed to stage files.",
			}
		}
	}

	run.StopSpinner()

	// Show staged summary for files staged in this run
	stagedAfter, _ := git.DiffNumstat(run, true)
	if stagedAfter == nil {
		stagedAfter = make(map[string]git.FileDiffStat)
	}

	filesThisRun := make(map[string]bool)
	targetList := filesToStage
	if hasSpecificFiles {
		targetList = opts.Files
	}
	for _, f := range targetList {
		filesThisRun[f] = true
	}
	if opts.Patch {
		for f, after := range stagedAfter {
			before := stagedBefore[f]
			if after.Insertions != before.Insertions || after.Deletions != before.Deletions || after.Binary != before.Binary {
				filesThisRun[f] = true
			}
		}
	}

	totalIns := 0
	totalDel := 0
	numFiles := 0

	for f := range filesThisRun {
		after, inAfter := stagedAfter[f]
		if !inAfter {
			continue
		}
		numFiles++
		before := stagedBefore[f]
		insDelta := after.Insertions - before.Insertions
		delDelta := after.Deletions - before.Deletions
		if insDelta > 0 {
			totalIns += insDelta
		}
		if delDelta > 0 {
			totalDel += delDelta
		}
	}

	if numFiles == 0 {
		fmt.Println("Nothing staged.")
		return nil
	}

	fileWord := "files"
	if numFiles == 1 {
		fileWord = "file"
	}
	diffStatStr := ""
	if totalIns > 0 && totalDel > 0 {
		diffStatStr = fmt.Sprintf(" (%s | %s)", ui.Success(fmt.Sprintf("+%d", totalIns)), ui.Danger(fmt.Sprintf("-%d", totalDel)))
	} else if totalIns > 0 {
		diffStatStr = fmt.Sprintf(" (%s)", ui.Success(fmt.Sprintf("+%d", totalIns)))
	} else if totalDel > 0 {
		diffStatStr = fmt.Sprintf(" (%s)", ui.Danger(fmt.Sprintf("-%d", totalDel)))
	}

	fmt.Printf("Staged %d %s%s\n", numFiles, fileWord, diffStatStr)
	return nil
}

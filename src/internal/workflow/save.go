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

// SaveOptions holds parameters for gx save.
type SaveOptions struct {
	Files           []string
	Message         string
	All             bool
	Patch           bool
	AllowSensitive  bool
	AllowEmpty      bool
	ExcludePatterns []string
}

// Save stages changes if needed and creates a commit.
func Save(run runner.Executor, cfg *config.Config, opts SaveOptions) error {
	// 1. Pre-flight: block if rebase or merge in progress
	if err := requireNoInProgress(inspect(run)); err != nil {
		return err
	}

	hasStaged, stagedErr := git.HasStagedChanges(run)
	if stagedErr != nil {
		return &cli.Error{
			Message: "Failed to check staged changes.",
			Hint:    "Run `git status` to inspect the index.",
		}
	}

	hasSpecificFiles := len(opts.Files) > 0

	// 2. Determine if staging is needed:
	// - Explicit specific files requested
	// - Patch mode requested
	// - -a / --all requested
	// - Or nothing is staged yet and working tree has changes
	needStaging := hasSpecificFiles || opts.Patch || opts.All || !hasStaged

	if needStaging {
		var filesToStage []string
		scannedTree := false
		var treeModified, treeUntracked []string

		if hasSpecificFiles {
			filesToStage = opts.Files
		} else if opts.Patch && len(opts.ExcludePatterns) == 0 {
			filesToStage = nil
		} else {
			var err error
			treeModified, treeUntracked, err = git.Status(run)
			if err != nil {
				return &cli.Error{
					Message: "Failed to scan working tree.",
				}
			}
			scannedTree = true

			// Filter out submodule paths
			submodules, _ := git.SubmodulePaths(run)
			if len(submodules) > 0 {
				var filteredModified, filteredUntracked []string
				for _, f := range treeModified {
					if submodules[f] {
						run.Info(fmt.Sprintf("  [submodule] %s (skipped — use git add manually)", f))
					} else {
						filteredModified = append(filteredModified, f)
					}
				}
				for _, f := range treeUntracked {
					if submodules[f] {
						run.Info(fmt.Sprintf("  [submodule] %s (skipped — use git add manually)", f))
					} else {
						filteredUntracked = append(filteredUntracked, f)
					}
				}
				treeModified = filteredModified
				treeUntracked = filteredUntracked
			}

			if !opts.Patch {
				if len(treeModified) > 0 || len(treeUntracked) > 0 {
					unstagedStats, _ := git.DiffNumstat(run, false)
					stagedStats, _ := git.DiffNumstat(run, true)

					var previewLines []string
					previewLines = append(previewLines, "Modified tracked files:")
					if len(treeModified) == 0 {
						previewLines = append(previewLines, "  (none)")
					} else {
						for _, f := range treeModified {
							stat := unstagedStats[f]
							if st, ok := stagedStats[f]; ok {
								stat.Insertions += st.Insertions
								stat.Deletions += st.Deletions
								if st.Binary {
									stat.Binary = true
								}
							}
							statStr := git.FormatDiffStat(stat)
							if statStr != "" {
								previewLines = append(previewLines, fmt.Sprintf("  %s %s", f, statStr))
							} else {
								previewLines = append(previewLines, fmt.Sprintf("  %s", f))
							}
						}
					}

					previewLines = append(previewLines, "", "New untracked files:")
					if len(treeUntracked) == 0 {
						previewLines = append(previewLines, "  (none)")
					} else {
						for _, f := range treeUntracked {
							previewLines = append(previewLines, fmt.Sprintf("  %s", f))
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
						filesToStage = append(treeModified, treeUntracked...)
					} else {
						filesToStage = treeModified
					}
				}
			} else {
				filesToStage = append(treeModified, treeUntracked...)
			}
		}

		// Apply --exclude patterns
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
			} else if scannedTree {
				checkFiles = filesToStage
			} else {
				modified, untracked, _ := git.Status(run)
				checkFiles = append(modified, untracked...)
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

		// Stage
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
	}

	// 3. Refresh staged changes state
	hasStaged, stagedErr = git.HasStagedChanges(run)
	if stagedErr != nil {
		run.StopSpinner()
		return &cli.Error{
			Message: "Failed to check staged changes.",
			Hint:    "Run `git status` to inspect the index.",
		}
	}

	// 4. Sensitive pattern check on all staged files (even pre-staged)
	if !opts.AllowSensitive && hasStaged {
		ps, _ := git.GetParsedStatus(run)
		if ps != nil {
			var allStaged []string
			for _, files := range ps.StagedFiles {
				allStaged = append(allStaged, files...)
			}
			matches := git.MatchSensitivePatterns(allStaged, cfg.SensitivePatterns)
			if len(matches) > 0 {
				run.StopSpinner()
				msg := "Sensitive file(s) detected in staging area — refusing to commit:\n"
				for _, m := range matches {
					msg += fmt.Sprintf("  %s\n", m)
				}
				return &cli.Error{
					Message: strings.TrimRight(msg, "\n"),
					Hint:    "Unstage the files, or use --allow-sensitive to override.",
				}
			}
		}
	}

	// 5. Empty commit guard
	if !opts.AllowEmpty && !hasStaged && !git.NonEmptyStatus(run) {
		run.StopSpinner()
		confirmed, err := prompts.Confirm("Nothing staged — commit anyway?")
		if err != nil {
			return err
		}
		if !confirmed {
			return &cli.Error{
				Message: "Save aborted.",
				Hint:    "Stage files first with gx stage, or use --allow-empty to commit with nothing staged.",
			}
		}
	}

	// 6. Conflict marker detection
	if hasStaged {
		run.StopSpinner()
		if err := checkConflictMarkers(run); err != nil {
			return err
		}
	}

	// 7. Inspect staged state before commit for the commit card
	ps, _ := git.GetParsedStatus(run)
	var stagedFiles map[string][]string
	if ps != nil {
		stagedFiles = ps.StagedFiles
	}
	stagedStats, _ := git.DiffNumstat(run, true)

	// 8. Commit
	if opts.Message != "" {
		run.State("Committing...")
	} else {
		run.StopSpinner()
	}

	stagedHint := "Check the error above. Your changes remain staged."
	noStagedHint := "Check the error above. You may try again with gx save."

	var commitErr error
	if opts.Message != "" {
		if hasStaged {
			commitErr = git.Commit(run, opts.Message)
		} else {
			commitErr = git.CommitAllowEmpty(run, opts.Message)
		}
	} else if hasStaged {
		commitErr = git.CommitEditor(run)
	} else {
		commitErr = git.CommitAllowEmptyEditor(run)
	}

	run.StopSpinner()

	if commitErr != nil {
		hint := noStagedHint
		if hasStaged {
			hint = stagedHint
		}
		return &cli.Error{
			Message: "Commit failed.",
			Hint:    hint,
		}
	}

	commitHash, _ := run.Output("rev-parse", "--short", "HEAD")
	commitSubject, _ := run.Output("log", "-1", "--format=%s")
	branch, _ := git.CurrentBranch(run)
	if branch == "" {
		branch = "detached HEAD"
	}

	fmt.Println(formatSaveCommitSummary(commitHash, branch, commitSubject, stagedFiles, stagedStats))
	return nil
}

func formatSaveCommitSummary(hash, branch, subject string, stagedFiles map[string][]string, stats map[string]git.FileDiffStat) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Saved commit %s (%s)\n", hash, branch))
	b.WriteString(fmt.Sprintf("%q", subject))

	totalIns := 0
	totalDel := 0
	totalFiles := 0

	categories := []struct {
		key   string
		title string
	}{
		{"A", "New files:"},
		{"M", "Modified:"},
		{"D", "Deleted:"},
		{"R", "Renamed:"},
	}

	for _, cat := range categories {
		files, ok := stagedFiles[cat.key]
		if !ok || len(files) == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("\n\n%s", cat.title))
		for _, f := range files {
			totalFiles++
			statStr := ""
			if stat, ok := stats[f]; ok {
				totalIns += stat.Insertions
				totalDel += stat.Deletions
				statStr = git.FormatDiffStat(stat)
			}
			if statStr != "" {
				b.WriteString(fmt.Sprintf("\n  %s %s", f, statStr))
			} else {
				b.WriteString(fmt.Sprintf("\n  %s", f))
			}
		}
	}

	b.WriteString("\n\n")
	if totalFiles == 0 {
		b.WriteString("0 files committed")
	} else {
		fileWord := "files"
		if totalFiles == 1 {
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
		b.WriteString(fmt.Sprintf("%d %s committed%s", totalFiles, fileWord, diffStatStr))
	}

	return b.String()
}

// checkConflictMarkers runs git diff --cached --check to detect unresolved
// conflict markers in staged files. If found, prompts the user to confirm.
func checkConflictMarkers(run runner.Executor) error {
	out, err := git.CheckCachedDiff(run)
	if err == nil || out == "" {
		return nil
	}
	conflicted := make(map[string]bool)
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(strings.ToLower(line), "conflict marker") {
			continue
		}
		if idx := strings.Index(line, ":"); idx > 0 {
			conflicted[line[:idx]] = true
		}
	}
	if len(conflicted) == 0 {
		return nil
	}

	fmt.Println("Warning: Unresolved conflict markers detected in staged files:")
	for f := range conflicted {
		fmt.Printf("  %s\n", f)
	}
	confirmed, err := prompts.Confirm("Commit with unresolved conflict markers?")
	if err != nil {
		return err
	}
	if !confirmed {
		return &cli.Error{
			Message: "Save aborted — resolve conflict markers first.",
			Hint:    "Resolve the conflicts, stage the files, then run gx save again.",
		}
	}
	return nil
}

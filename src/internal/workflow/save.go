package workflow

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/fathss/gx/internal/cli"
	"github.com/fathss/gx/internal/config"
	"github.com/fathss/gx/internal/git"
	"github.com/fathss/gx/internal/runner"
)

// Save stages changes and creates a commit.
//
// Args:
//   - run: git command runner
//   - cfg: current configuration (uses SensitivePatterns)
//   - files: specific file paths to stage (empty = stage all)
//   - msg: commit message (empty = open editor)
//   - patchMode: if true, use git add -p instead of auto-staging
//   - allowSensitive: if true, skip sensitive-pattern check
//   - allowEmpty: if true, skip empty-commit confirmation
//   - excludePatterns: glob patterns for files to exclude from staging
func Save(run *runner.Runner, cfg *config.Config, files []string, msg string, patchMode bool, allowSensitive bool, allowEmpty bool, excludePatterns []string) error {
	// 1. Pre-flight: block if rebase or merge in progress
	if git.IsRebaseInProgress(run) {
		return &cli.Error{
			Message: "A rebase is already in progress.",
			Hint:    "Resolve or abort the rebase first.\nUse gx sync --continue or git rebase --abort/--continue.",
		}
	}
	if git.IsMergeInProgress(run) {
		return &cli.Error{
			Message: "A merge is already in progress.",
			Hint:    "Resolve or abort the merge first.\nUse gx sync --continue or git merge --abort/--continue.",
		}
	}

	// 2. Determine which files to stage
	var filesToStage []string
	hasSpecificFiles := len(files) > 0
	scannedTree := false
	var treeModified, treeUntracked []string

	if hasSpecificFiles {
		// Specific files provided — use them directly, skip untracked prompt
		filesToStage = files
	} else if patchMode && len(excludePatterns) == 0 {
		// Issue 10a: Pure patch mode — no file list needed, adds interactively
		// Scan will be done separately for sensitive check if needed.
		filesToStage = nil
	} else {
		// No args — scan working tree (also for patch+exclude or non-patch)
		var err error
		treeModified, treeUntracked, err = git.Status(run)
		if err != nil {
			return &cli.Error{
				Message: "Failed to scan working tree.",
			}
		}
		scannedTree = true

		// Issue 13: Detect and filter submodule paths
		submodules, _ := git.SubmodulePaths(run)
		var filteredModified, filteredUntracked []string
		for _, f := range treeModified {
			if submodules[f] {
				fmt.Printf("  [submodule] %s (skipped — use git add manually)\n", f)
			} else {
				filteredModified = append(filteredModified, f)
			}
		}
		for _, f := range treeUntracked {
			if submodules[f] {
				fmt.Printf("  [submodule] %s (skipped — use git add manually)\n", f)
			} else {
				filteredUntracked = append(filteredUntracked, f)
			}
		}
		treeModified = filteredModified
		treeUntracked = filteredUntracked

		if !patchMode {
			// Print categorized preview (skip in patch+exclude mode — user selects hunks interactively)
			fmt.Println("Modified tracked files:")
			if len(treeModified) == 0 {
				fmt.Println("  (none)")
			} else {
				for _, f := range treeModified {
					fmt.Printf("  %s\n", f)
				}
			}

			fmt.Println("\nNew untracked files:")
			if len(treeUntracked) == 0 {
				fmt.Println("  (none)")
			} else {
				for _, f := range treeUntracked {
					fmt.Printf("  %s\n", f)
				}
			}

			// Prompt for untracked files
			var stageUntracked bool
			if len(treeUntracked) > 0 {
				stageUntracked, err = promptConfirm(fmt.Sprintf("Stage %d untracked file(s)?", len(treeUntracked)))
				if err != nil {
					return err
				}
			}

			if stageUntracked {
				filesToStage = append(treeModified, treeUntracked...)
			} else {
				filesToStage = treeModified
			}
		} else {
			// Issue 10a: patch+exclude — use filtered file list for interactive staging
			filesToStage = append(treeModified, treeUntracked...)
		}
	}

	// 2b. Apply --exclude patterns (removes matching files before staging and sensitive check)
	if len(excludePatterns) > 0 {
		filesToStage = git.FilterExcluded(filesToStage, excludePatterns)
		if hasSpecificFiles {
			files = git.FilterExcluded(files, excludePatterns)
			// All specific files were excluded — fall through to empty-commit guard
			if len(files) == 0 {
				hasSpecificFiles = false
				filesToStage = nil
			}
		}
	}

	// 3. Sensitive pattern check
	if !allowSensitive {
		var checkFiles []string
		if hasSpecificFiles {
			checkFiles = files
		} else if filesToStage != nil {
			checkFiles = filesToStage
		} else if scannedTree {
			// filesToStage was filtered by exclude — use it (may be nil/empty)
			checkFiles = filesToStage
		} else {
			// Issue 10b: Pure patch mode without scan yet — do a quick scan
			modified, untracked, _ := git.Status(run)
			checkFiles = append(modified, untracked...)
		}
		matches := git.MatchSensitivePatterns(checkFiles, cfg.SensitivePatterns)
		if len(matches) > 0 {
			if patchMode {
				// Issue 10b: In patch mode, warn but don't block
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
					Message: strings.TrimSpace(msg),
					Hint:    "Remove or gitignore the files, or use --allow-sensitive to override.",
				}
			}
		}
	}

	// Issue 14: Warn before overwriting partial staging (no-args, non-patch, non-patch+exclude)
	// Only warn when there are BOTH staged changes AND unstaged tracked-file changes.
	if !hasSpecificFiles && !patchMode && len(filesToStage) > 0 {
		hasUnstaged, _ := git.DiffUnstagedFiles(run)
		if git.HasStagedChanges(run) && hasUnstaged != "" {
			warned, err := promptConfirm("Some files are already staged. Running gx save will stage all remaining changes in tracked files, overwriting any partial staging. Continue?")
			if err != nil {
				return err
			}
			if !warned {
				return &cli.Error{
					Message: "Save aborted.",
				}
			}
		}
	}

	// 4. Stage
	if patchMode {
		if len(excludePatterns) > 0 {
			// All files excluded — skip staging, empty-commit guard below handles it
			if len(filesToStage) == 0 {
				// fall through to empty-commit guard
			} else if err := git.AddPatch(run, filesToStage...); err != nil {
				return &cli.Error{
					Message: "Patch staging failed.",
					Hint:    "Check the error above and try again.",
				}
			}
		} else {
			if err := git.AddPatch(run, files...); err != nil {
				return &cli.Error{
					Message: "Patch staging failed.",
					Hint:    "Check the error above and try again.",
				}
			}
		}
	} else if hasSpecificFiles {
		if err := git.Add(run, files...); err != nil {
			return &cli.Error{
				Message: "Failed to stage files.",
			}
		}
	} else {
		if len(filesToStage) > 0 {
			if err := git.Add(run, filesToStage...); err != nil {
				return &cli.Error{
					Message: "Failed to stage files.",
				}
			}
		}
	}

	// 5. Show diffstat
	diffStat, err := git.DiffStatCached(run)
	if err != nil {
		// Non-fatal: diffstat is informational
		fmt.Println("(could not compute diffstat)")
	} else if diffStat != "" {
		fmt.Println(diffStat)
	}

	// 6. Empty commit guard
	if !allowEmpty && !git.NonEmptyStatus(run) {
		confirmed, err := promptConfirm("Nothing staged — commit anyway?")
		if err != nil {
			return err
		}
		if !confirmed {
			return &cli.Error{
				Message: "Save aborted.",
				Hint:    "Stage files first, or use --allow-empty to commit with nothing staged.",
			}
		}
	}

	// 7. Conflict marker detection (Issue 15)
	hasStaged := git.HasStagedChanges(run)
	if hasStaged {
		if err := checkConflictMarkers(run); err != nil {
			return err
		}
	}

	// Issue 12: Differentiate hint based on whether changes were staged
	stagedHint := "Check the error above. Your changes remain staged."
	noStagedHint := "Check the error above. You may try again with gx save."

	// 8. Commit
	if msg != "" {
		// With -m flag
		if hasStaged {
			if err := git.Commit(run, msg); err != nil {
				return &cli.Error{
					Message: "Commit failed.",
					Hint:    stagedHint,
				}
			}
		} else {
			if err := git.CommitAllowEmpty(run, msg); err != nil {
				return &cli.Error{
					Message: "Commit failed.",
					Hint:    noStagedHint,
				}
			}
		}
	} else if hasStaged {
		// Without -m flag, with staged changes — open editor
		if err := git.CommitEditor(run); err != nil {
			return &cli.Error{
				Message: "Commit failed.",
				Hint:    stagedHint,
			}
		}
	} else {
		// Without -m flag, no staged changes — allow-empty via editor
		if err := git.CommitAllowEmptyEditor(run); err != nil {
			return &cli.Error{
				Message: "Commit failed.",
				Hint:    noStagedHint,
			}
		}
	}

	return nil
}

// checkConflictMarkers runs git diff --cached --check to detect unresolved
// conflict markers in staged files. If found, prompts the user to confirm.
func checkConflictMarkers(run *runner.Runner) error {
	out, err := git.CheckCachedDiff(run)
	if err == nil || out == "" {
		return nil
	}
	// Parse conflicted file paths from output lines like:
	//   file.txt:7: leftover conflict marker
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
	confirmed, err := promptConfirm("Commit with unresolved conflict markers?")
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

// promptConfirm asks the user a yes/no question on the terminal.
// Returns true if the user answered y or Y. Returns false for n, N, or empty.
func promptConfirm(prompt string) (bool, error) {
	fmt.Printf("%s [y/N]: ", prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return false, &cli.Error{
				Message: fmt.Sprintf("Failed to read input: %s.", err),
			}
		}
		// EOF (e.g. pipe) — treat as no
		return false, nil
	}
	answer := strings.TrimSpace(scanner.Text())
	return strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes"), nil
}

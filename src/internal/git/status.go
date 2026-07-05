package git

import (
	"fmt"
	"strings"

	"github.com/fathss/gx/internal/runner"
)

// StatusLabels maps porcelain status letters to human-readable labels.
var StatusLabels = map[string]string{
	"M": "modified",
	"A": "new file",
	"D": "deleted",
	"R": "renamed",
}

// ParsedStatus holds a structured representation of git status --porcelain.
type ParsedStatus struct {
	StagedFiles     map[string][]string // indexed by status letter (M, A, D, R)
	UnstagedFiles   []string
	UntrackedFiles  []string
	ConflictedFiles []string
}

// GetParsedStatus returns a structured parse of the repository status.
// Uses separate git commands for each category to avoid the `runner.Output`
// trimming issue with `git status --porcelain` (leading space on first line
// is stripped, corrupting the status code).
func GetParsedStatus(run *runner.Runner) (*ParsedStatus, error) {
	ps := &ParsedStatus{
		StagedFiles: make(map[string][]string),
	}

	// 1. Staged files: git diff --cached --diff-filter=ACDMR --name-status
	//    Output format: M<tab><path> or A<tab><path>, etc.
	//    Renames: R<NN><tab><old><tab><new>
	stagedOut, err := run.Output("diff", "--cached", "--diff-filter=ACDMR", "--name-status")
	if err != nil {
		return nil, err
	}
	if stagedOut != "" {
		for _, line := range strings.Split(stagedOut, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			// Tab-separated: <status>\t<path> or <status>\t<old>\t<new> (rename)
			parts := strings.SplitN(line, "\t", 3)
			if len(parts) < 2 {
				continue
			}

			statusLetter := string(parts[0][0])
			path := parts[len(parts)-1] // last part = destination path (handles renames)

			if _, ok := StatusLabels[statusLetter]; ok {
				ps.StagedFiles[statusLetter] = append(ps.StagedFiles[statusLetter], path)
			}
		}
	}

	// 2. Unstaged tracked-file changes: git diff --name-only
	//    Output format: <path> (one per line)
	unstagedOut, err := run.Output("diff", "--name-only")
	if err != nil {
		return nil, err
	}
	if unstagedOut != "" {
		for _, line := range strings.Split(unstagedOut, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				ps.UnstagedFiles = append(ps.UnstagedFiles, line)
			}
		}
	}

	// 3. Untracked files: git ls-files --others --exclude-standard
	ps.UntrackedFiles, err = UntrackedFiles(run)
	if err != nil {
		return nil, err
	}

	// 4. Conflicted files: git diff --name-only --diff-filter=U
	ps.ConflictedFiles = ConflictedFiles(run)

	return ps, nil
}

// FormatStagedLabel returns the human-readable label for a porcelain status letter.
// Returns the raw letter if no label is known.
func FormatStagedLabel(letter string) string {
	if label, ok := StatusLabels[letter]; ok {
		return label
	}
	return fmt.Sprintf("status %s", letter)
}

// Status parses git status --porcelain and returns lists of modified tracked
// files and untracked files.
//
// Standard porcelain v1 format: XY<space>PATH or XY<space>ORIG_PATH -> PATH
// XY is a 2-character status code. runner.Output trims leading whitespace
// from the full output, which can strip the leading space from the first
// porcelain line (X=' ' = unmodified in index). The parsing handles this
// by always reading the path from line[2:] — the separator space is naturally
// absorbed whether the leading space was trimmed or not.
// For renames (R/C status), extracts only the destination path after " -> ".
func Status(run *runner.Runner) (modified []string, untracked []string, err error) {
	out, err := run.Output("status", "--porcelain")
	if err != nil || out == "" {
		return nil, nil, err
	}
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		if len(line) < 2 {
			continue
		}

		status := line[:2]
		path := strings.TrimSpace(line[2:])

		// Handle renames/copies: ORIG_PATH -> PATH — extract destination
		if idx := strings.LastIndex(path, " -> "); idx != -1 {
			path = path[idx+4:]
		}

		if strings.HasPrefix(status, "?") {
			untracked = append(untracked, path)
		} else {
			modified = append(modified, path)
		}
	}
	return modified, untracked, nil
}

// NonEmptyStatus returns true when git status --porcelain has any output for
// tracked files (staged or unstaged). Untracked files are ignored.
func NonEmptyStatus(run *runner.Runner) bool {
	out, err := run.Output("status", "--porcelain", "--untracked-files=no")
	return err == nil && out != ""
}

// SubmodulePaths returns a set of paths that are git submodules.
// Detected by scanning git ls-files --stage for 160000 (gitlink) mode entries.
func SubmodulePaths(run *runner.Runner) (map[string]bool, error) {
	out, err := run.Output("ls-files", "--stage")
	if err != nil {
		return nil, err
	}
	submodules := make(map[string]bool)
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "160000") {
			// Format: "160000 <hash> 0\t<path>"
			parts := strings.SplitN(line, "\t", 2)
			if len(parts) == 2 {
				submodules[parts[1]] = true
			}
		}
	}
	return submodules, nil
}

// UntrackedFiles returns a list of untracked files that are not gitignored.
func UntrackedFiles(run *runner.Runner) ([]string, error) {
	out, err := run.Output("ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

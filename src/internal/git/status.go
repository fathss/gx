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
	ModifiedFiles   []string // every non-untracked entry, in porcelain order (Status()'s view)
}

// parsePorcelain parses `git status --porcelain -uall` output. Single
// classification point for both exported views: GetParsedStatus buckets by
// category, Status() reads ModifiedFiles/UntrackedFiles.
//
// Format: XY<space>PATH, or XY<space>ORIG_PATH -> PATH for renames — the
// separator sits at index 2, so the leading status column (X may be a
// space) is preserved by the runner's right-trim and always readable.
// Unmerged entries (U*, AA, DD) belong to ConflictedFiles only.
func parsePorcelain(out string) *ParsedStatus {
	ps := &ParsedStatus{
		StagedFiles: make(map[string][]string),
	}
	if out == "" {
		return ps
	}
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		x, y := line[0], line[1]
		path := line[3:]
		// Handle renames/copies: ORIG_PATH -> PATH — extract destination.
		if idx := strings.LastIndex(path, " -> "); idx != -1 {
			path = path[idx+4:]
		}

		switch {
		case x == '?' && y == '?':
			ps.UntrackedFiles = append(ps.UntrackedFiles, path)
		case x == 'U' || y == 'U' || (x == 'A' && y == 'A') || (x == 'D' && y == 'D'):
			ps.ConflictedFiles = append(ps.ConflictedFiles, path)
			ps.ModifiedFiles = append(ps.ModifiedFiles, path)
		default:
			ps.ModifiedFiles = append(ps.ModifiedFiles, path)
			if _, ok := StatusLabels[string(x)]; ok {
				ps.StagedFiles[string(x)] = append(ps.StagedFiles[string(x)], path)
			}
			if y == 'M' || y == 'D' {
				ps.UnstagedFiles = append(ps.UnstagedFiles, path)
			}
		}
	}
	return ps
}

// statusPorcelain is the single query both exported views parse.
func statusPorcelain(run runner.Executor) (string, error) {
	return run.Output("status", "--porcelain", "--untracked-files=all")
}

// GetParsedStatus returns a structured parse of the repository status.
func GetParsedStatus(run runner.Executor) (*ParsedStatus, error) {
	out, err := statusPorcelain(run)
	if err != nil {
		return nil, err
	}
	return parsePorcelain(out), nil
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
// Shares parsePorcelain with GetParsedStatus, so the two views can never
// disagree about what is staged, untracked, or conflicted. Renames/copies
// contribute only their destination path.
func Status(run runner.Executor) (modified []string, untracked []string, err error) {
	out, err := statusPorcelain(run)
	if err != nil || out == "" {
		return nil, nil, err
	}
	ps := parsePorcelain(out)
	return ps.ModifiedFiles, ps.UntrackedFiles, nil
}

// NonEmptyStatus returns true when git status --porcelain has any output for
// tracked files (staged or unstaged). Untracked files are ignored.
func NonEmptyStatus(run runner.Executor) bool {
	out, err := run.Output("status", "--porcelain", "--untracked-files=no")
	return err == nil && out != ""
}

// SubmodulePaths returns a set of paths that are git submodules.
// Detected by scanning git ls-files --stage for 160000 (gitlink) mode entries.
func SubmodulePaths(run runner.Executor) (map[string]bool, error) {
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

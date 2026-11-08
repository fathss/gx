package git

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/fathss/gx/internal/runner"
)

// FileDiffStat represents line additions and deletions for a single file.
type FileDiffStat struct {
	Insertions int
	Deletions  int
	Binary     bool
}

// DiffStatCached returns the diffstat of staged changes.
// Equivalent to `git diff --stat --cached`.
func DiffStatCached(run runner.Executor) (string, error) {
	return run.Output("diff", "--stat", "--cached")
}

// HasStagedChanges reports whether the index has staged changes.
// `git diff --cached --quiet` exits 1 when differences exist and 0 when the
// index is clean; any other failure is returned as an error so callers never
// mistake a broken repository for "nothing staged".
func HasStagedChanges(run runner.Executor) (bool, error) {
	_, err := run.Output("diff", "--cached", "--quiet")
	if err == nil {
		return false, nil
	}
	var cmdErr *runner.CommandError
	if errors.As(err, &cmdErr) && cmdErr.ExitCode == 1 {
		return true, nil
	}
	return false, err
}

// DiffUnstagedFiles returns the names of tracked files that have unstaged modifications.
func DiffUnstagedFiles(run runner.Executor) (string, error) {
	return run.Output("diff", "--name-only")
}

// CheckCachedDiff runs git diff --cached --check and returns combined output.
// Used to detect conflict markers in staged files.
func CheckCachedDiff(run runner.Executor) (string, error) {
	return run.CombinedOutput("diff", "--cached", "--check")
}

// DiffNumstat returns a map of file path to FileDiffStat.
// If cached is true, it runs `git diff --numstat --cached` (for staged files).
// If cached is false, it runs `git diff --numstat` (for unstaged files).
func DiffNumstat(run runner.Executor, cached bool) (map[string]FileDiffStat, error) {
	args := []string{"diff", "--numstat"}
	if cached {
		args = append(args, "--cached")
	}
	out, err := run.Output(args...)
	if err != nil {
		return nil, err
	}
	return ParseNumstat(out), nil
}

// ParseNumstat parses git diff --numstat output into a map of filepath to FileDiffStat.
func ParseNumstat(out string) map[string]FileDiffStat {
	result := make(map[string]FileDiffStat)
	if out == "" {
		return result
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: <insertions>\t<deletions>\t<path>
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		insStr, delStr, rawPath := parts[0], parts[1], parts[2]
		path := parseNumstatPath(rawPath)

		if insStr == "-" && delStr == "-" {
			result[path] = FileDiffStat{Binary: true}
			continue
		}

		ins, _ := strconv.Atoi(insStr)
		del, _ := strconv.Atoi(delStr)
		result[path] = FileDiffStat{
			Insertions: ins,
			Deletions:  del,
			Binary:     false,
		}
	}
	return result
}

func parseNumstatPath(raw string) string {
	raw = strings.Trim(raw, "\"")
	if idx := strings.Index(raw, "{"); idx != -1 {
		endIdx := strings.Index(raw, "}")
		if endIdx != -1 && endIdx > idx {
			inside := raw[idx+1 : endIdx]
			if arrowIdx := strings.Index(inside, " => "); arrowIdx != -1 {
				newPart := inside[arrowIdx+4:]
				return raw[:idx] + newPart + raw[endIdx+1:]
			}
		}
	}
	if arrowIdx := strings.LastIndex(raw, " => "); arrowIdx != -1 {
		return raw[arrowIdx+4:]
	}
	return raw
}

// FormatDiffStat formats a FileDiffStat as a human-readable string like "(+10, -2)".
func FormatDiffStat(stat FileDiffStat) string {
	if stat.Binary {
		return "(binary)"
	}
	if stat.Insertions > 0 && stat.Deletions > 0 {
		return fmt.Sprintf("(+%d, -%d)", stat.Insertions, stat.Deletions)
	}
	if stat.Insertions > 0 {
		return fmt.Sprintf("(+%d)", stat.Insertions)
	}
	if stat.Deletions > 0 {
		return fmt.Sprintf("(-%d)", stat.Deletions)
	}
	return ""
}

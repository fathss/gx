package git

import (
	"errors"

	"github.com/fathss/gx/internal/runner"
)

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

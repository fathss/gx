package git

import "github.com/fathss/gx/internal/runner"

// DiffStatCached returns the diffstat of staged changes.
// Equivalent to `git diff --stat --cached`.
func DiffStatCached(run *runner.Runner) (string, error) {
	return run.Output("diff", "--stat", "--cached")
}

// HasStagedChanges returns true when there are staged changes in the index.
// Uses git diff --cached --quiet which exits 0 if nothing is staged.
func HasStagedChanges(run *runner.Runner) bool {
	_, err := run.Output("diff", "--cached", "--quiet")
	return err != nil
}

// DiffUnstagedFiles returns the names of tracked files that have unstaged modifications.
func DiffUnstagedFiles(run *runner.Runner) (string, error) {
	return run.Output("diff", "--name-only")
}

// CheckCachedDiff runs git diff --cached --check and returns combined output.
// Used to detect conflict markers in staged files.
func CheckCachedDiff(run *runner.Runner) (string, error) {
	return run.CombinedOutput("diff", "--cached", "--check")
}

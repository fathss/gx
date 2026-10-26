package git

import "github.com/fathss/gx/internal/runner"

// Commit creates a commit with the given message.
// Equivalent to `git commit -m <msg>`.
func Commit(run runner.Executor, msg string) error {
	return run.Run("commit", "-m", msg)
}

// CommitEditor creates a commit by opening the configured $EDITOR.
// Equivalent to `git commit`.
func CommitEditor(run runner.Executor) error {
	return run.Run("commit")
}

// CommitAllowEmpty creates an empty commit with the given message.
// Equivalent to `git commit --allow-empty -m <msg>`.
func CommitAllowEmpty(run runner.Executor, msg string) error {
	return run.Run("commit", "--allow-empty", "-m", msg)
}

// CommitAllowEmptyEditor creates an empty commit by opening $EDITOR.
// Equivalent to `git commit --allow-empty`.
func CommitAllowEmptyEditor(run runner.Executor) error {
	return run.Run("commit", "--allow-empty")
}

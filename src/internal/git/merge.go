package git

import (
	"strings"

	"github.com/fathss/gx/internal/runner"
)

// Merge integrates the specified branch into the current branch.
// Uses --no-edit to suppress the editor — gx workflows handle messages.
// Equivalent to `git merge --no-edit <branch>`.
func Merge(run *runner.Runner, branch string) error {
	return run.Run("merge", "--no-edit", branch)
}

// IsMergeInProgress returns true if a merge is currently paused with conflicts.
// Uses git rev-parse to verify MERGE_HEAD exists (exits 0 when present).
func IsMergeInProgress(run *runner.Runner) bool {
	_, err := run.Output("rev-parse", "-q", "--verify", "MERGE_HEAD")
	return err == nil
}

// MergeContinue completes a paused merge after conflicts are resolved and staged.
// GIT_EDITOR=true suppresses the editor prompt — git uses the auto-generated
// merge message when no editor is invoked.
func MergeContinue(run *runner.Runner) error {
	return run.RunWithEnv([]string{"GIT_EDITOR=true"}, "merge", "--continue")
}

// MergeAbort aborts a paused merge and restores the pre-merge state.
func MergeAbort(run *runner.Runner) error {
	return run.Run("merge", "--abort")
}

// ConflictedFiles returns the list of files with unresolved merge conflicts.
// Uses git diff --name-only --diff-filter=U. Returns nil if none or on error.
// Works for conflicts from both merge and rebase operations.
func ConflictedFiles(run *runner.Runner) []string {
	out, err := run.Output("diff", "--name-only", "--diff-filter=U")
	if err != nil || out == "" {
		return nil
	}
	lines := strings.Split(out, "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	return lines
}

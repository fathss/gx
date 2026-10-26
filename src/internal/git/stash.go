package git

import (
	"errors"
	"fmt"
	"strings"

	"github.com/fathss/gx/internal/runner"
)

// IsIndexLocked reports whether err is a stash failure caused by another
// process holding git's index lock. The generic "commit or discard" hint is
// wrong for a transient lock contention.
func IsIndexLocked(err error) bool {
	var cmdErr *runner.CommandError
	if !errors.As(err, &cmdErr) {
		return false
	}
	return strings.Contains(cmdErr.Stderr, "index.lock") ||
		strings.Contains(cmdErr.Stderr, "Unable to create")
}

// StashList returns all stash entries as lines from git stash list.
// Returns nil if there are no stashes or on error.
func StashList(run runner.Executor) ([]string, error) {
	out, err := run.Output("stash", "list")
	if err != nil || out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// Distinct stash-label prefixes per command — each command that stashes uses
// its own prefix so HasGXStash only matches stashes from the same command.
const SyncStashPrefix = "gx-sync/"

// StashPush stashes tracked-file changes with a descriptive message label.
func StashPush(run runner.Executor, message string) error {
	return run.Run("stash", "push", "-m", message)
}

// StashPop restores the stash entry scoped to branch (or any gx-sync/ entry
// when branch is empty). Uses the stash reference (stash@{N}) so it pops the
// correct entry even when the user has pushed or popped other stashes in
// between. Returns nil if no matching stash is found — the entry was already
// dealt with.
func StashPop(run runner.Executor, branch ...string) error {
	ref, err := findStashByPrefix(run, optionalBranch(branch))
	if err != nil || ref == "" {
		return nil
	}
	return run.Run("stash", "pop", ref)
}

func optionalBranch(branch []string) string {
	if len(branch) > 0 {
		return branch[0]
	}
	return ""
}

// stashMatches reports whether a stash subject was created for branch.
// Labels look like gx-sync/<branch>/<unix-timestamp>. Matching the exact
// branch (not just its prefix) keeps feature/a from matching feature/a/b.
// An empty branch matches every gx-sync/ entry.
func stashMatches(subject, branch string) bool {
	if !strings.HasPrefix(subject, SyncStashPrefix) {
		return false
	}
	if branch == "" {
		return true
	}
	prefix := SyncStashPrefix + branch + "/"
	if !strings.HasPrefix(subject, prefix) {
		return false
	}
	stamp := strings.TrimPrefix(subject, prefix)
	if stamp == "" {
		return false
	}
	for _, r := range stamp {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// findStashByPrefix returns the stash reference (e.g. "stash@{2}") of the most
// recent stash entry matching branch (any gx-sync/ entry when branch is empty).
// Returns "" if none found.
func findStashByPrefix(run runner.Executor, branch string) (string, error) {
	out, err := run.Output("stash", "list")
	if err != nil || out == "" {
		return "", nil
	}
	for _, line := range strings.Split(out, "\n") {
		// Format: "stash@{N}: ...: <message>"
		parts := strings.SplitN(line, ": ", 3)
		if len(parts) == 3 && stashMatches(parts[2], branch) {
			return strings.TrimSpace(parts[0]), nil
		}
	}
	return "", nil
}

// DropGXStash drops all stash entries matching branch (or all gx-sync/
// entries when branch is empty). Used for cleanup of orphaned gx entries.
func DropGXStash(run runner.Executor, branch ...string) error {
	for {
		ref, err := findStashByPrefix(run, optionalBranch(branch))
		if err != nil || ref == "" {
			return nil
		}
		if err := run.Run("stash", "drop", ref); err != nil {
			return fmt.Errorf("failed to drop %s: %w", ref, err)
		}
	}
}

// IsClean returns true when no tracked files have staged or unstaged changes.
// Untracked files are ignored — they don't block checkout or rebase.
func IsClean(run runner.Executor) bool {
	out, err := run.Output("status", "--porcelain", "--untracked-files=no")
	return err == nil && out == ""
}

// HasGXStash returns true if a gx-sync stash exists for branch (or any
// gx-sync stash when branch is empty).
func HasGXStash(run runner.Executor, branch ...string) bool {
	ref, err := findStashByPrefix(run, optionalBranch(branch))
	return err == nil && ref != ""
}

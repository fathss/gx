package git

import (
	"fmt"
	"strings"

	"github.com/fathss/gx/internal/runner"
)

// StashList returns all stash entries as lines from git stash list.
// Returns nil if there are no stashes or on error.
func StashList(run *runner.Runner) ([]string, error) {
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
func StashPush(run *runner.Runner, message string) error {
	return run.Run("stash", "push", "-m", message)
}

// StashPop restores the stash entry matching SyncStashPrefix.
// Uses the stash reference (stash@{N}) so it pops the correct entry even when
// the user has pushed or popped other stashes in between. Returns nil if no
// matching stash is found — the entry was already dealt with.
func StashPop(run *runner.Runner) error {
	ref, err := findStashByPrefix(run, SyncStashPrefix)
	if err != nil || ref == "" {
		return nil
	}
	return run.Run("stash", "pop", ref)
}

// findStashByPrefix returns the stash reference (e.g. "stash@{2}") of the most
// recent stash entry whose message starts with prefix. Returns "" if none found.
func findStashByPrefix(run *runner.Runner, prefix string) (string, error) {
	out, err := run.Output("stash", "list")
	if err != nil || out == "" {
		return "", nil
	}
	for _, line := range strings.Split(out, "\n") {
		// Format: "stash@{N}: ...: <message>"
		parts := strings.SplitN(line, ": ", 3)
		if len(parts) == 3 && strings.HasPrefix(parts[2], prefix) {
			return strings.TrimSpace(parts[0]), nil
		}
	}
	return "", nil
}

// DropGXStash drops all stash entries matching SyncStashPrefix.
// Used for cleanup of orphaned gx entries.
func DropGXStash(run *runner.Runner) error {
	for {
		ref, err := findStashByPrefix(run, SyncStashPrefix)
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
func IsClean(run *runner.Runner) bool {
	out, err := run.Output("status", "--porcelain", "--untracked-files=no")
	return err == nil && out == ""
}

// HasGXStash returns true if any stash entry was created by gx sync
// (label starts with SyncStashPrefix). Used to distinguish gx-orchestrated
// rebases from manual ones.
func HasGXStash(run *runner.Runner) bool {
	ref, err := findStashByPrefix(run, SyncStashPrefix)
	return err == nil && ref != ""
}

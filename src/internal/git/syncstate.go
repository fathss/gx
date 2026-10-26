package git

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/fathss/gx/internal/runner"
)

// gxSyncStateFile is written into the git directory while gx owns an
// in-progress rebase or merge. The content is the branch that owns it.
const gxSyncStateFile = "gx-sync-state"

// GitDir returns the absolute path of the git directory for this worktree.
func GitDir(run runner.Executor) (string, error) {
	return run.Output("rev-parse", "--absolute-git-dir")
}

// WriteGXSyncState records that gx started the paused operation for branch.
// A clean working tree creates no stash, so this marker — not stash
// presence — is the authoritative ownership signal.
func WriteGXSyncState(run runner.Executor, branch string) error {
	dir, err := GitDir(run)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, gxSyncStateFile), []byte(branch+"\n"), 0644)
}

// ReadGXSyncState returns the branch recorded by WriteGXSyncState.
func ReadGXSyncState(run runner.Executor) (string, bool) {
	dir, err := GitDir(run)
	if err != nil {
		return "", false
	}
	data, err := os.ReadFile(filepath.Join(dir, gxSyncStateFile))
	if err != nil {
		return "", false
	}
	branch := strings.TrimSpace(string(data))
	if branch == "" {
		return "", false
	}
	return branch, true
}

// ClearGXSyncState removes the gx ownership marker. Missing is not an error.
func ClearGXSyncState(run runner.Executor) error {
	dir, err := GitDir(run)
	if err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(dir, gxSyncStateFile)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// RebaseBranch returns the branch being rebased, read from the rebase state
// files. HEAD is detached during a rebase, so CurrentBranch cannot supply it.
// Returns "" when no rebase is in progress.
func RebaseBranch(run runner.Executor) string {
	dir, err := GitDir(run)
	if err != nil {
		return ""
	}
	for _, backend := range []string{"rebase-merge", "rebase-apply"} {
		data, err := os.ReadFile(filepath.Join(dir, backend, "head-name"))
		if err != nil {
			continue
		}
		name := strings.TrimSpace(string(data))
		return strings.TrimPrefix(name, "refs/heads/")
	}
	return ""
}

package workflow

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fathss/gx/internal/cli"
	"github.com/fathss/gx/internal/git"
	"github.com/fathss/gx/internal/runner"
)

// syncLockFile serializes mutating gx sync runs for one worktree. Two syncs
// racing on stash/checkout/rebase otherwise fail on git's index.lock with a
// misleading "commit or discard your changes" hint.
const syncLockFile = "gx-sync.lock"

// acquireSyncLock takes an exclusive, non-blocking lock in the git directory.
// The returned release function is safe to call more than once. A lock whose
// PID is no longer alive (a crashed run) is reclaimed.
func acquireSyncLock(run runner.Executor) (func(), error) {
	dir, err := git.GitDir(run)
	if err != nil {
		return nil, &cli.Error{
			Message: "Failed to locate the Git directory.",
			Hint:    "Check that you are inside a Git repository.",
		}
	}
	path := filepath.Join(dir, syncLockFile)

	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
			_ = f.Sync()
			_ = f.Close()
			released := false
			return func() {
				if released {
					return
				}
				released = true
				releaseSyncLock(path)
			}, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, &cli.Error{
				Message: "Failed to acquire the sync lock.",
				Hint:    "Check that the repository is writable, then retry.",
			}
		}
		if !syncLockStale(path) {
			return nil, &cli.Error{
				Message: "Another gx sync is already running in this repository.",
				Hint:    "Wait for it to finish, then retry.",
			}
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, &cli.Error{
				Message: "Failed to clear a stale sync lock.",
				Hint:    "Remove .git/gx-sync.lock manually, then retry.",
			}
		}
	}

	return nil, &cli.Error{
		Message: "Another gx sync is already running in this repository.",
		Hint:    "Wait for it to finish, then retry.",
	}
}

func syncLockStale(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return true
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return lockOlderThan(path, 10*time.Second)
	}
	if !processAlive(pid) {
		return true
	}
	// Platforms that cannot probe another process age the lock out instead.
	if !processAliveReliable() {
		return lockOlderThan(path, 24*time.Hour)
	}
	return false
}

func lockOlderThan(path string, age time.Duration) bool {
	info, err := os.Stat(path)
	if err != nil {
		return true
	}
	return time.Since(info.ModTime()) > age
}

func releaseSyncLock(path string) {
	if data, err := os.ReadFile(path); err == nil {
		if pid, perr := strconv.Atoi(strings.TrimSpace(string(data))); perr == nil && pid != os.Getpid() {
			return // the lock now belongs to another process
		}
	}
	_ = os.Remove(path)
}

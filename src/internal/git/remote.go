package git

import (
	"errors"
	"strings"

	"github.com/fathss/gx/internal/runner"
)

// IsNonFastForward reports whether err is a failed git command whose stderr
// shows the remote rejected the update because it wasn't a fast-forward.
// It is the one evidence-backed push diagnosis; every other failure stays
// cause-agnostic (git's stderr is already streamed to the user).
func IsNonFastForward(err error) bool {
	var cmdErr *runner.CommandError
	if !errors.As(err, &cmdErr) {
		return false
	}
	return strings.Contains(cmdErr.Stderr, "non-fast-forward")
}

func Fetch(run runner.Executor, remote string) error {
	return run.Run("fetch", remote)
}

func FastForward(run runner.Executor, remote, branch string) error {
	return run.Run("merge", "--ff-only", Qualify(remote, branch))
}

// RemoteExists returns true if the given remote name is configured.
func RemoteExists(run runner.Executor, name string) bool {
	_, err := run.Output("remote", "get-url", name)
	return err == nil
}

// RemoteBranchExists returns true if the remote-tracking branch ref exists.
func RemoteBranchExists(run runner.Executor, remote, branch string) bool {
	_, err := run.Output("rev-parse", "--verify", Qualify(remote, branch))
	return err == nil
}

// RemoteHEAD returns the branch that the remote's HEAD symbolic-ref points to,
// e.g. "main" for origin/HEAD -> origin/main. Returns "" if the remote HEAD
// cannot be resolved (new remote, detached remote HEAD, etc.).
func RemoteHEAD(run runner.Executor, remote string) string {
	out, err := run.Output("symbolic-ref", "refs/remotes/"+remote+"/HEAD")
	if err != nil || out == "" {
		return ""
	}
	// out is like "refs/remotes/origin/main" — strip the remote prefix
	trimmed := strings.TrimPrefix(out, "refs/remotes/"+remote+"/")
	return trimmed
}

// Push pushes the current branch to remote without setting upstream.
func Push(run runner.Executor, remote, branch string) error {
	return run.Run("push", remote, branch)
}

// PushSetUpstream pushes and sets upstream tracking (-u).
func PushSetUpstream(run runner.Executor, remote, branch string) error {
	return run.Run("push", "-u", remote, branch)
}

// PushForceWithLease force-pushes with lease (safe force).
func PushForceWithLease(run runner.Executor, remote, branch string) error {
	return run.Run("push", "--force-with-lease", remote, branch)
}

// RemoteURL returns the URL of the given remote.
func RemoteURL(run runner.Executor, remote string) (string, error) {
	return run.Output("remote", "get-url", remote)
}

// DeleteRemoteBranch deletes a branch on the remote with
// `git push <remote> --delete <branch>`.
func DeleteRemoteBranch(run runner.Executor, remote, branch string) error {
	return run.Run("push", remote, "--delete", branch)
}

package git

import (
	"fmt"
	"strings"

	"github.com/fathss/gx/internal/runner"
)

func Fetch(run *runner.Runner, remote string) error {
	return run.Run("fetch", remote)
}

func FastForward(run *runner.Runner, remote, branch string) error {
	target := fmt.Sprintf("%s/%s", remote, branch)
	return run.Run("merge", "--ff-only", target)
}

// RemoteExists returns true if the given remote name is configured.
func RemoteExists(run *runner.Runner, name string) bool {
	_, err := run.Output("remote", "get-url", name)
	return err == nil
}

// RemoteBranchExists returns true if the remote-tracking branch ref exists.
func RemoteBranchExists(run *runner.Runner, remote, branch string) bool {
	_, err := run.Output("rev-parse", "--verify", remote+"/"+branch)
	return err == nil
}

// RemoteHEAD returns the branch that the remote's HEAD symbolic-ref points to,
// e.g. "main" for origin/HEAD -> origin/main. Returns "" if the remote HEAD
// cannot be resolved (new remote, detached remote HEAD, etc.).
func RemoteHEAD(run *runner.Runner, remote string) string {
	out, err := run.Output("symbolic-ref", "refs/remotes/"+remote+"/HEAD")
	if err != nil || out == "" {
		return ""
	}
	// out is like "refs/remotes/origin/main" — strip the remote prefix
	trimmed := strings.TrimPrefix(out, "refs/remotes/"+remote+"/")
	return trimmed
}

// Push pushes the current branch to remote without setting upstream.
func Push(run *runner.Runner, remote, branch string) error {
	return run.Run("push", remote, branch)
}

// PushSetUpstream pushes and sets upstream tracking (-u).
func PushSetUpstream(run *runner.Runner, remote, branch string) error {
	return run.Run("push", "-u", remote, branch)
}

// PushForceWithLease force-pushes with lease (safe force).
func PushForceWithLease(run *runner.Runner, remote, branch string) error {
	return run.Run("push", "--force-with-lease", remote, branch)
}

// HasUpstream returns true if the branch has an upstream tracking branch configured.
func HasUpstream(run *runner.Runner, branch string) bool {
	_, err := run.Output("rev-parse", "--abbrev-ref", branch+"@{upstream}")
	return err == nil
}

// RemoteURL returns the URL of the given remote.
func RemoteURL(run *runner.Runner, remote string) (string, error) {
	return run.Output("remote", "get-url", remote)
}

// DeleteRemoteBranch deletes a branch on the remote with
// `git push <remote> --delete <branch>`.
func DeleteRemoteBranch(run *runner.Runner, remote, branch string) error {
	return run.Run("push", remote, "--delete", branch)
}

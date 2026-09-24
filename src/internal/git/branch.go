package git

import (
	"fmt"
	"strings"

	"github.com/fathss/gx/internal/runner"
)

// HEADState holds information about the current HEAD position.
type HEADState struct {
	Branch     string // "" when detached
	ShortHash  string // abbreviated hash, "" if error
	IsDetached bool
}

// GetHEADState returns the current HEAD state: branch name (or detached), short hash.
func GetHEADState(run *runner.Runner) (*HEADState, error) {
	branch, err := CurrentBranch(run)
	if err != nil {
		return nil, err
	}

	hash, err := ShortHeadHash(run)
	if err != nil {
		return nil, err
	}

	return &HEADState{
		Branch:     branch,
		ShortHash:  hash,
		IsDetached: branch == "",
	}, nil
}

// ShortHeadHash returns the abbreviated hash of HEAD (7 chars).
// Returns "" if HEAD cannot be resolved (e.g. empty repository with no commits).
func ShortHeadHash(run *runner.Runner) (string, error) {
	out, err := run.Output("rev-parse", "--short", "HEAD")
	if err != nil {
		return "", nil // empty repo — no commits yet
	}
	return out, nil
}

func CurrentBranch(run *runner.Runner) (string, error) {
	return run.Output("branch", "--show-current")
}

func Checkout(run *runner.Runner, branch string) error {
	return run.Run("checkout", branch)
}

// LocalBranchExists returns true if the given local branch exists.
func LocalBranchExists(run *runner.Runner, branch string) bool {
	_, err := run.Output("rev-parse", "--verify", "refs/heads/"+branch)
	return err == nil
}

// MergedBranches returns local branches that are merged into the given base.
// It wraps `git branch --merged <base>`.
func MergedBranches(run *runner.Runner, base string) ([]string, error) {
	out, err := run.Output("branch", "--merged", base)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return nil, nil
	}
	var branches []string
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "* ") {
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "* "))
		}
		if trimmed != "" {
			branches = append(branches, trimmed)
		}
	}
	return branches, nil
}

// MergedRemoteBranches returns remote-tracking refs that are merged into the
// given base. It wraps `git branch -r --merged <base>` where base is the
// qualified remote base (e.g. "origin/develop"). The remote parameter is used
// to qualify the base when it is not already qualified.
func MergedRemoteBranches(run *runner.Runner, remote, base string) ([]string, error) {
	qualified := base
	if !strings.HasPrefix(base, remote+"/") {
		qualified = fmt.Sprintf("%s/%s", remote, base)
	}
	out, err := run.Output("branch", "-r", "--merged", qualified)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return nil, nil
	}
	var refs []string
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		refs = append(refs, trimmed)
	}
	return refs, nil
}

// DeleteLocalBranch deletes a local branch with `git branch -d` (safe delete).
func DeleteLocalBranch(run *runner.Runner, branch string) error {
	return run.Run("branch", "-d", branch)
}

// DeleteRemoteTrackingBranch deletes a remote-tracking ref with
// `git branch -d -r <remote>/<branch>`. The branch argument may be a bare
// name or an already-qualified ref; the remote prefix is normalized.
func DeleteRemoteTrackingBranch(run *runner.Runner, remote, branch string) error {
	bare := branch
	if strings.HasPrefix(branch, remote+"/") {
		bare = strings.TrimPrefix(branch, remote+"/")
	}
	ref := fmt.Sprintf("%s/%s", remote, bare)
	return run.Run("branch", "-d", "-r", ref)
}

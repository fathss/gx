package git

import (
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
func GetHEADState(run runner.Executor) (*HEADState, error) {
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
func ShortHeadHash(run runner.Executor) (string, error) {
	out, err := run.Output("rev-parse", "--short", "HEAD")
	if err != nil {
		return "", nil // empty repo — no commits yet
	}
	return out, nil
}

func CurrentBranch(run runner.Executor) (string, error) {
	return run.Output("branch", "--show-current")
}

func Checkout(run runner.Executor, branch string) error {
	return run.Run("checkout", branch)
}

// LocalBranchExists returns true if the given local branch exists.
func LocalBranchExists(run runner.Executor, branch string) bool {
	_, err := run.Output("rev-parse", "--verify", "refs/heads/"+branch)
	return err == nil
}

// MergedBranches returns local branches that are merged into the given base.
// It wraps `git branch --merged <base>`.
func MergedBranches(run runner.Executor, base string) ([]string, error) {
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
		if trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, " ") {
			continue
		}
		branches = append(branches, trimmed)
	}
	return branches, nil
}

// MergedRemoteBranches returns the bare names of remote-tracking branches
// merged into the given base. It wraps `git branch -r --merged <base>`; the
// base is qualified with remote when not already qualified. Normalization
// happens here so workflows never see git listing formats: the "<remote>/"
// prefix, symbolic-ref arrow lines ("origin/HEAD -> origin/main"), and
// origin/HEAD itself are resolved or dropped below the seam.
func MergedRemoteBranches(run runner.Executor, remote, base string) ([]string, error) {
	out, err := run.Output("branch", "-r", "--merged", Qualify(remote, base))
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
		if strings.Contains(trimmed, "->") {
			continue
		}
		if !strings.HasPrefix(trimmed, remote+"/") {
			continue
		}
		bare := StripRemote(remote, trimmed)
		if bare == "" || bare == "HEAD" {
			continue
		}
		branches = append(branches, bare)
	}
	return branches, nil
}

// DeleteLocalBranch deletes a local branch with `git branch -d` (safe delete).
func DeleteLocalBranch(run runner.Executor, branch string) error {
	return run.Run("branch", "-d", branch)
}

// DeleteRemoteTrackingBranch deletes a remote-tracking ref with
// `git branch -d -r <remote>/<branch>`. The branch argument may be a bare
// name or an already-qualified ref; the remote prefix is normalized.
func DeleteRemoteTrackingBranch(run runner.Executor, remote, branch string) error {
	ref := Qualify(remote, StripRemote(remote, branch))
	return run.Run("branch", "-d", "-r", ref)
}

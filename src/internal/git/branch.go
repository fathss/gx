package git

import "github.com/fathss/gx/internal/runner"

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
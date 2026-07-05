package git

import (
	"fmt"
	"strings"

	"github.com/fathss/gx/internal/runner"
)

// DetectRemote returns the remote tracking the current branch, or the first
// configured remote. Returns an error if no remotes are configured.
func DetectRemote(run *runner.Runner) (string, error) {
	branch, err := run.Output("branch", "--show-current")
	if err == nil && branch != "" {
		remote, err := run.Output("config", "branch."+branch+".remote")
		if err == nil && remote != "" {
			return remote, nil
		}
	}

	remotes, err := run.Output("remote")
	if err != nil {
		return "", err
	}
	parts := strings.Fields(remotes)
	if len(parts) == 0 {
		return "", fmt.Errorf("no remotes configured")
	}
	return parts[0], nil
}

// DetectDefaultBranch detects the remote's HEAD branch. Tries:
//  1. <remote>/HEAD symbolic ref (local, set by clone)
//  2. Common branch names: main, master, develop
func DetectDefaultBranch(run *runner.Runner, remote string) (string, error) {
	ref, err := run.Output("rev-parse", "--abbrev-ref", remote+"/HEAD")
	if err == nil && ref != "" {
		branch := strings.TrimPrefix(ref, remote+"/")
		if branch != "" && branch != remote+"/HEAD" {
			return branch, nil
		}
	}

	for _, name := range []string{"main", "master", "develop"} {
		if _, err := run.Output("rev-parse", "--verify", name); err == nil {
			return name, nil
		}
	}

	return "", fmt.Errorf("could not detect default branch")
}

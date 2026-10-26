package git

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fathss/gx/internal/runner"
)

// Divergence describes how a local branch relates to its remote-tracking ref
// at <remote>/<branch>. Tracking reports whether that ref exists at all;
// Ahead and Behind are only meaningful when Tracking is true.
type Divergence struct {
	Ahead    int
	Behind   int
	Tracking bool
}

// CheckDivergence compares the current branch against <remote>/<branch>.
// A missing remote-tracking ref is not an error: it returns
// Divergence{Tracking: false}. Actual git failures are returned as errors so
// callers never mistake a broken repository for "no upstream".
func CheckDivergence(run runner.Executor, remote, branch string) (Divergence, error) {
	if !RemoteBranchExists(run, remote, branch) {
		return Divergence{}, nil
	}
	out, err := run.Output("rev-list", "--count", "--left-right", Qualify(remote, branch)+"...HEAD")
	if err != nil {
		return Divergence{}, err
	}
	parts := strings.Fields(out)
	if len(parts) != 2 {
		return Divergence{}, fmt.Errorf("failed to parse ahead/behind counts: %s", out)
	}
	// git rev-list --count --left-right outputs <behind>\t<ahead>
	// Left side = remote ref (commits remote has that local doesn't = behind)
	// Right side = HEAD (commits local has that remote doesn't = ahead)
	behind, errB := strconv.Atoi(parts[0])
	ahead, errA := strconv.Atoi(parts[1])
	if errA != nil || errB != nil {
		return Divergence{}, fmt.Errorf("failed to parse ahead/behind counts: %s", out)
	}
	return Divergence{Ahead: ahead, Behind: behind, Tracking: true}, nil
}

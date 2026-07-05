package git

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fathss/gx/internal/runner"
)

// AheadBehind returns the number of commits the local branch is ahead of and
// behind its remote-tracking counterpart at <remote>/<branch>.
// Returns -1, -1, nil if the remote-tracking ref doesn't exist (no upstream).
func AheadBehind(run *runner.Runner, remote, branch string) (ahead, behind int, err error) {
	out, err := run.Output("rev-list", "--count", "--left-right", fmt.Sprintf("%s/%s...HEAD", remote, branch))
	if err != nil {
		return -1, -1, nil // no upstream yet
	}
	parts := strings.Fields(out)
	if len(parts) != 2 {
		return 0, 0, nil
	}
	// git rev-list --count --left-right outputs <behind>\t<ahead>
	// Left side = remote ref (commits remote has that local doesn't = behind)
	// Right side = HEAD (commits local has that remote doesn't = ahead)
	behind, errA := strconv.Atoi(parts[0])
	ahead, errB := strconv.Atoi(parts[1])
	if errA != nil || errB != nil {
		return 0, 0, fmt.Errorf("failed to parse ahead/behind counts: %s", out)
	}
	return ahead, behind, nil
}

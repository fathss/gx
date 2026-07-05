package git

import (
	"fmt"
	"strings"

	"github.com/fathss/gx/internal/runner"
)

// CommitInfo holds information about a single commit.
type CommitInfo struct {
	ShortHash    string // abbreviated hash (7 chars)
	RelativeDate string // e.g. "2 hours ago"
	Subject      string // first line of commit message
}

// RecentCommits returns the last n commits on the current branch.
// Returns an empty slice if there are no commits.
func RecentCommits(run *runner.Runner, n int) ([]CommitInfo, error) {
	format := "%h|||%ar|||%s"
	out, err := run.Output("log", fmt.Sprintf("-n %d", n), fmt.Sprintf("--format=%s", format))
	if err != nil || out == "" {
		return nil, nil
	}

	var commits []CommitInfo
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|||", 3)
		if len(parts) != 3 {
			continue
		}
		hash := strings.TrimSpace(parts[0])
		date := strings.TrimSpace(parts[1])
		subject := strings.TrimSpace(parts[2])

		if hash != "" && subject != "" {
			commits = append(commits, CommitInfo{
				ShortHash:    hash,
				RelativeDate: date,
				Subject:      subject,
			})
		}
	}

	return commits, nil
}

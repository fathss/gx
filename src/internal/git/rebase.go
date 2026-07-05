package git

import (
	"fmt"
	"os"
	"strings"

	"github.com/fathss/gx/internal/runner"
)

func Rebase(run *runner.Runner, branch string) error {
	return run.Run("rebase", branch)
}

// RebaseAbort aborts an in-progress rebase and restores the original branch.
func RebaseAbort(run *runner.Runner) error {
	return run.Run("rebase", "--abort")
}

// RebaseContinue continues an in-progress rebase after conflicts are resolved.
// GIT_EDITOR=true suppresses the editor prompt — the original commit message
// from the replayed commit is reused automatically.
func RebaseContinue(run *runner.Runner) error {
	return run.RunWithEnv([]string{"GIT_EDITOR=true"}, "rebase", "--continue")
}

// RebaseSkip skips the currently-applying commit during a rebase.
// GIT_EDITOR=true suppresses the editor prompt.
func RebaseSkip(run *runner.Runner) error {
	return run.RunWithEnv([]string{"GIT_EDITOR=true"}, "rebase", "--skip")
}

// IsRebaseInProgress checks whether a rebase is currently in progress.
// Uses .git/rebase-merge or .git/rebase-apply directory detection —
// reliable during all rebase phases (editing, applying, paused, stopped).
// Does NOT use --show-current-patch which returns false negatives during
// interactive edit pauses, --skip on last patch, or --stop.
func IsRebaseInProgress(run *runner.Runner) bool {
	gitDir, err := run.Output("rev-parse", "--git-dir")
	if err != nil {
		return false
	}
	// Merge backend (default since git 2.26)
	if _, err := os.Stat(gitDir + "/rebase-merge"); err == nil {
		return true
	}
	// Apply backend (older git or --apply)
	if _, err := os.Stat(gitDir + "/rebase-apply"); err == nil {
		return true
	}
	return false
}

// CurrentRebasePatchInfo returns a short human-readable description of the
// currently applying commit during a rebase, e.g. "'Fix login bug' (abc1234)".
// Returns empty string if no rebase is in progress or info cannot be parsed.
//
// Handles two git output formats:
//   - merge backend (default since git 2.26): "commit <hash>" + indented subject
//   - apply backend (older git):                "From <hash>" + "Subject: [PATCH] <subject>"
func CurrentRebasePatchInfo(run *runner.Runner) string {
	out, err := run.Output("rebase", "--show-current-patch")
	if err != nil || out == "" {
		return ""
	}

	var hash, subject string
	lines := strings.Split(out, "\n")

	// First pass: extract hash from "commit" or "From" line, and subject from "Subject:" line
	for _, line := range lines {
		if strings.HasPrefix(line, "commit ") && hash == "" {
			hash = strings.TrimSpace(strings.TrimPrefix(line, "commit "))
			if len(hash) > 7 {
				hash = hash[:7]
			}
		} else if strings.HasPrefix(line, "From ") && hash == "" {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				hash = parts[1]
				if len(hash) > 7 {
					hash = hash[:7]
				}
			}
		} else if strings.HasPrefix(line, "Subject: ") && subject == "" {
			subject = strings.TrimPrefix(line, "Subject: ")
			subject = strings.TrimPrefix(subject, "[PATCH] ")
			subject = strings.TrimSpace(subject)
		}
	}

	// Second pass: if no "Subject:" line (merge-backend format), extract the
	// first indented line after the metadata block as the subject.
	if subject == "" {
		seenBlank := false
		for _, line := range lines {
			if !seenBlank {
				if strings.TrimSpace(line) == "" {
					seenBlank = true
				}
				continue
			}
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(line, "diff ") {
				break
			}
			if subject == "" {
				subject = trimmed
			}
		}
	}

	if subject != "" && hash != "" {
		return fmt.Sprintf("'%s' (%s)", subject, hash)
	}
	if hash != "" {
		return hash
	}
	return subject
}


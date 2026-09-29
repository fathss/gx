package workflow

import (
	"github.com/fathss/gx/internal/cli"
	"github.com/fathss/gx/internal/git"
	"github.com/fathss/gx/internal/runner"
)

// qVerdict is the classification of the repository's sync-relevant state —
// the six rows of the Q-table (see docs/commands/sync.md). The signals → Q
// mapping exists exactly once, in classify().
type qVerdict int

const (
	q1Idle         qVerdict = iota // idle — nothing paused, no gx stash
	q2OrphanStash                  // gx-sync/ stash, nothing paused
	q3ManualRebase                 // manual rebase — gx refuses to touch
	q4GXRebase                     // gx-orchestrated rebase
	q5ManualMerge                  // manual merge — gx refuses to touch
	q6GXMerge                      // gx-orchestrated merge
)

// classify maps raw signals to a verdict. Order mirrors the historical
// switch: rebase takes precedence on the impossible rebase&&merge input.
func classify(rebase, merge, gxStash bool) qVerdict {
	switch {
	case rebase && gxStash:
		return q4GXRebase
	case merge && gxStash:
		return q6GXMerge
	case rebase:
		return q3ManualRebase
	case merge:
		return q5ManualMerge
	case gxStash:
		return q2OrphanStash
	default:
		return q1Idle
	}
}

// inspect reads the three git signals and classifies them. Trivial glue —
// callers hoist a single call to function entry.
func inspect(run *runner.Runner) qVerdict {
	return classify(
		git.IsRebaseInProgress(run),
		git.IsMergeInProgress(run),
		git.HasGXStash(run),
	)
}

// requireNoInProgress is the shared guard copy for workflows that block on
// any paused rebase/merge (save, ship). Q1 and Q2 are unblocked.
func requireNoInProgress(verdict qVerdict) error {
	switch verdict {
	case q3ManualRebase, q4GXRebase:
		return &cli.Error{
			Message: "A rebase is already in progress.",
			Hint:    "Resolve or abort the rebase first.\nUse gx sync --continue or git rebase --abort/--continue.",
		}
	case q5ManualMerge, q6GXMerge:
		return &cli.Error{
			Message: "A merge is already in progress.",
			Hint:    "Resolve or abort the merge first.\nUse gx sync --continue or git merge --abort/--continue.",
		}
	default:
		return nil
	}
}

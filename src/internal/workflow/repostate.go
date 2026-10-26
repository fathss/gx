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
// gxOwned means gx started the paused operation (ownership marker or, for
// older paused operations, a branch-scoped gx stash).
func classify(rebase, merge, gxOwned bool) qVerdict {
	switch {
	case rebase && gxOwned:
		return q4GXRebase
	case merge && gxOwned:
		return q6GXMerge
	case rebase:
		return q3ManualRebase
	case merge:
		return q5ManualMerge
	case gxOwned:
		return q2OrphanStash
	default:
		return q1Idle
	}
}

// inspectForSync classifies the repository and returns the branch that owns
// any gx state plus whether the ownership marker exists. The marker is the
// authoritative signal: a clean working tree creates no stash, so a paused
// gx rebase/merge would otherwise be indistinguishable from a manual one.
func inspectForSync(run runner.Executor, current string) (qVerdict, string, bool) {
	rebase := git.IsRebaseInProgress(run)
	merge := git.IsMergeInProgress(run)

	markerBranch, hasMarker := git.ReadGXSyncState(run)
	branch := current
	if branch == "" && rebase {
		// HEAD is detached while rebasing; recover the branch from git's
		// rebase state so stash lookups stay branch-scoped.
		branch = git.RebaseBranch(run)
	}
	if hasMarker {
		branch = markerBranch
	}

	// The marker covers the clean-tree case. The stash check remains for
	// compatibility with operations paused by an older gx that wrote no
	// marker; it is always branch-scoped.
	owned := hasMarker || (branch != "" && git.HasGXStash(run, branch))
	return classify(rebase, merge, owned), branch, hasMarker
}

// inspect classifies the repository for guards that only need to know whether
// any rebase/merge is paused (save, ship). Q-table mapping lives in classify().
func inspect(run runner.Executor) qVerdict {
	current, _ := git.CurrentBranch(run)
	verdict, _, _ := inspectForSync(run, current)
	return verdict
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

package workflow

import (
	"fmt"
	"time"

	"github.com/fathss/gx/internal/cli"
	"github.com/fathss/gx/internal/git"
	"github.com/fathss/gx/internal/runner"
)

// syncGit is the narrow capability set a sync session needs from the git
// layer. Two adapters satisfy it: prodSyncGit in production, an in-memory
// fake in tests — two adapters means the seam is real.
type syncGit interface {
	StashPush(label string) error
	StashPop() error
	DropGXStash() error
	IsClean() bool
	HasGXStash() bool
	Warnf(msg, hint string)
}

// prodSyncGit adapts the git package and runner to syncGit.
type prodSyncGit struct {
	run *runner.Runner
}

func newProdSyncGit(run *runner.Runner) syncGit {
	return &prodSyncGit{run: run}
}

func (p *prodSyncGit) StashPush(label string) error { return git.StashPush(p.run, label) }
func (p *prodSyncGit) StashPop() error              { return git.StashPop(p.run) }
func (p *prodSyncGit) DropGXStash() error           { return git.DropGXStash(p.run) }
func (p *prodSyncGit) IsClean() bool                { return git.IsClean(p.run) }
func (p *prodSyncGit) HasGXStash() bool             { return git.HasGXStash(p.run) }
func (p *prodSyncGit) Warnf(msg, hint string)       { p.run.Warnf(msg, hint) }

// settle is how a sync session closes (see CONTEXT.md).
type settle int

const (
	// settleCommit pops the stash and cleans up orphaned gx-sync/ entries.
	settleCommit settle = iota
	// settlePause keeps the stash — it powers --continue and --abort.
	settlePause
	// settleRestore pops the stash, returning the tree to its pre-sync state.
	settleRestore
)

// syncSession owns one stash lifecycle: begin acquires the working-tree
// stash, resume adopts an interrupted session's stash, and settle closes the
// session exactly once. The check → warn → act → recover contract's recover
// half lives here as an invariant, not as a call-site obligation.
type syncSession struct {
	git      syncGit
	acquired bool
}

// begin acquires a session: a clean tree stashes nothing; a dirty tree is
// stashed under a gx-sync/ label so settle can later restore or commit it.
func begin(g syncGit, branch string) (*syncSession, error) {
	s := &syncSession{git: g}
	if g.IsClean() {
		return s, nil
	}
	label := fmt.Sprintf("%s%s/%d", git.SyncStashPrefix, branch, time.Now().Unix())
	fmt.Println("Stashing local changes...")
	if err := g.StashPush(label); err != nil {
		return nil, &cli.Error{
			Message: "Failed to stash local changes.",
			Hint:    "Commit or discard your changes manually.",
		}
	}
	s.acquired = true
	return s, nil
}

// resume adopts an interrupted sync session: its gx-sync/ stash (if any)
// becomes this session's to settle. Used by --continue, --skip, and --abort,
// which run in a fresh process with no live session state.
func resume(g syncGit) *syncSession {
	return &syncSession{git: g, acquired: g.HasGXStash()}
}

// run executes the act-phase fn and settles the session exactly once:
//
//	(Commit, nil) -> commit: pop + orphan cleanup
//	(Pause, err)  -> pause: stash kept, err propagates
//	(_, err)      -> restore: pop, err propagates
//
// Every exit from fn settles — the invariant that used to be enforced by 11
// hand-placed restore calls is now the only thing run does. If restoring
// itself fails, the failure is surfaced as a warning so fn's original error
// still reaches the caller.
func (s *syncSession) run(fn func() (settle, error)) error {
	outcome, err := fn()
	if err != nil && outcome != settlePause {
		if serr := s.settle(settleRestore); serr != nil {
			if ce, ok := serr.(*cli.Error); ok {
				s.git.Warnf(ce.Message, ce.Hint)
			}
		}
		return err
	}
	if serr := s.settle(outcome); serr != nil {
		return serr
	}
	return err
}

// settle closes the session. Settle copy (the announcement, the conflict
// error) lives here and nowhere else.
func (s *syncSession) settle(out settle) error {
	switch out {
	case settlePause:
		return nil

	case settleCommit:
		if s.acquired {
			if err := s.pop(); err != nil {
				return err
			}
			s.acquired = false
		}
		if err := s.git.DropGXStash(); err != nil {
			s.git.Warnf("Failed to clean up gx stash entries.", fmt.Sprintf("Error: %v", err))
		}
		return nil

	default: // settleRestore
		if !s.acquired {
			return nil
		}
		if err := s.pop(); err != nil {
			return err
		}
		s.acquired = false
		return nil
	}
}

// pop announces the restore and pops the gx-sync/ stash. A pop conflict
// returns the standard settle error — the single place that message exists.
func (s *syncSession) pop() error {
	fmt.Println("Restoring stashed changes...")
	if err := s.git.StashPop(); err != nil {
		return &cli.Error{
			Message: "Stash pop failed — conflicts detected.",
			Hint:    "Resolve the conflicts above, then run: git stash drop",
		}
	}
	return nil
}

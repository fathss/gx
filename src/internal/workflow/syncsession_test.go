package workflow

import (
	"errors"
	"strings"
	"testing"

	"github.com/fathss/gx/internal/cli"
)

var errStub = errors.New("stub git failure")

// fakeSyncGit is the in-memory second adapter behind the syncGit seam.
type fakeSyncGit struct {
	clean   bool
	hasGX   bool
	pushErr error
	popErr  error
	dropErr error
	pushes  []string
	pops    int
	drops   int
	warns   [][2]string
}

func (f *fakeSyncGit) StashPush(label string) error {
	if f.pushErr != nil {
		return f.pushErr
	}
	f.pushes = append(f.pushes, label)
	f.hasGX = true
	return nil
}

func (f *fakeSyncGit) StashPop() error {
	if f.popErr != nil {
		return f.popErr
	}
	f.pops++
	f.hasGX = false
	return nil
}

func (f *fakeSyncGit) DropGXStash() error {
	f.drops++
	return f.dropErr
}

func (f *fakeSyncGit) IsClean() bool    { return f.clean }
func (f *fakeSyncGit) HasGXStash() bool { return f.hasGX }
func (f *fakeSyncGit) Warnf(msg, hint string) {
	f.warns = append(f.warns, [2]string{msg, hint})
}

// Seam test 1: begin acquires a session by stashing a dirty tree.
func TestBeginDirtysPushesLabeledStash(t *testing.T) {
	g := &fakeSyncGit{clean: false}

	s, err := begin(g, "feature/login")
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if !s.acquired {
		t.Fatal("session not acquired")
	}
	if len(g.pushes) != 1 {
		t.Fatalf("pushes = %d, want 1", len(g.pushes))
	}
	if !strings.HasPrefix(g.pushes[0], "gx-sync/feature/login/") {
		t.Fatalf("label = %q, want gx-sync/feature/login/ prefix", g.pushes[0])
	}
	if g.pops != 0 || g.drops != 0 {
		t.Fatalf("begin must not settle: pops=%d drops=%d", g.pops, g.drops)
	}
}

// Seam test 1b: a clean tree begins with nothing to acquire.
func TestBeginCleanAcquiresNothing(t *testing.T) {
	g := &fakeSyncGit{clean: true}

	s, err := begin(g, "main")
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if s.acquired {
		t.Fatal("clean tree must not acquire")
	}
	if len(g.pushes) != 0 {
		t.Fatalf("pushes = %d, want 0", len(g.pushes))
	}
}

// Seam test 2: an act-phase error settles as restore — pop runs, the
// original error is what the caller sees.
func TestRunErrorRestoresBeforeReturning(t *testing.T) {
	g := &fakeSyncGit{clean: false}
	s, err := begin(g, "feature")
	if err != nil {
		t.Fatalf("begin: %v", err)
	}

	sentinel := &cli.Error{Message: "Failed to fetch remote."}
	got := s.run(func() (settle, error) { return settleRestore, sentinel })

	if got != sentinel {
		t.Fatalf("run = %v, want original error", got)
	}
	if g.pops != 1 {
		t.Fatalf("pops = %d, want 1", g.pops)
	}
	if g.drops != 0 {
		t.Fatalf("drops = %d, restore must not orphan-clean", g.drops)
	}
}

// Seam test 3: a conflict pause keeps the stash — that is what --continue
// and --abort resume later.
func TestRunPauseKeepsStash(t *testing.T) {
	g := &fakeSyncGit{clean: false}
	s, err := begin(g, "feature")
	if err != nil {
		t.Fatalf("begin: %v", err)
	}

	pauseErr := &cli.Error{Message: "Rebase stopped because of conflicts."}
	got := s.run(func() (settle, error) { return settlePause, pauseErr })

	if got != pauseErr {
		t.Fatalf("run = %v, want pause error", got)
	}
	if g.pops != 0 {
		t.Fatalf("pops = %d, want 0", g.pops)
	}
	if g.drops != 0 {
		t.Fatalf("drops = %d, want 0", g.drops)
	}
	if !g.hasGX {
		t.Fatal("pause must keep the gx stash")
	}
}

// Seam test 4: commit pops the stash and cleans up gx-sync/ entries, both
// for a session that acquired its own stash and one resumed from disk.
func TestRunCommitPopsAndDrops(t *testing.T) {
	g := &fakeSyncGit{clean: false}
	s, err := begin(g, "feature")
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if got := s.run(func() (settle, error) { return settleCommit, nil }); got != nil {
		t.Fatalf("run = %v, want nil", got)
	}
	if g.pops != 1 || g.drops != 1 {
		t.Fatalf("fresh commit: pops=%d drops=%d, want 1/1", g.pops, g.drops)
	}

	g2 := &fakeSyncGit{clean: true, hasGX: true}
	if got := resume(g2).run(func() (settle, error) { return settleCommit, nil }); got != nil {
		t.Fatalf("resumed run = %v, want nil", got)
	}
	if g2.pops != 1 || g2.drops != 1 {
		t.Fatalf("resumed commit: pops=%d drops=%d, want 1/1", g2.pops, g2.drops)
	}
}

// Seam test 5: a pop conflict surfaces the standard settle copy, once.
func TestSettleRestoreConflictReturnsSettleCopy(t *testing.T) {
	g := &fakeSyncGit{clean: false, popErr: errStub}
	s, err := begin(g, "feature")
	if err != nil {
		t.Fatalf("begin: %v", err)
	}

	got := s.settle(settleRestore)
	ce, ok := got.(*cli.Error)
	if !ok {
		t.Fatalf("settle = %#v, want *cli.Error", got)
	}
	if !strings.Contains(ce.Message, "conflicts detected") {
		t.Fatalf("message = %q, want conflict copy", ce.Message)
	}
	if g.pops != 0 || g.drops != 0 {
		t.Fatalf("failed pop must not drop: pops=%d drops=%d", g.pops, g.drops)
	}
}

// Seam test 5b: when restore fails during an error-path settle, the caller
// still sees the original act-phase error and the conflict is reported as a
// warning instead of masking it.
func TestRunRestoreFailurePreservesOriginalError(t *testing.T) {
	g := &fakeSyncGit{clean: false, popErr: errStub}
	s, err := begin(g, "feature")
	if err != nil {
		t.Fatalf("begin: %v", err)
	}

	original := &cli.Error{Message: "Failed to fetch remote."}
	got := s.run(func() (settle, error) { return settleRestore, original })

	if got != original {
		t.Fatalf("run = %v, want original error", got)
	}
	if len(g.warns) != 1 || !strings.Contains(g.warns[0][0], "conflicts detected") {
		t.Fatalf("warns = %v, want one conflict warning", g.warns)
	}
}

// Seam test 6: resuming when no gx stash exists adopts nothing — the
// session pops nothing, and commit still attempts orphan cleanup.
func TestResumeWithoutStashPopsNothing(t *testing.T) {
	g := &fakeSyncGit{clean: true, hasGX: false}
	s := resume(g)

	if s.acquired {
		t.Fatal("resume without stash must not acquire")
	}
	if got := s.run(func() (settle, error) { return settleCommit, nil }); got != nil {
		t.Fatalf("run = %v, want nil", got)
	}
	if g.pops != 0 {
		t.Fatalf("pops = %d, want 0", g.pops)
	}
	if g.drops != 1 {
		t.Fatalf("drops = %d, want orphan cleanup even without stash", g.drops)
	}
}

// Restore settle on a session with nothing acquired is a no-op.
func TestSettleRestoreWithoutAcquireIsNoop(t *testing.T) {
	g := &fakeSyncGit{clean: true}
	s := resume(g)

	if got := s.settle(settleRestore); got != nil {
		t.Fatalf("settle = %v, want nil", got)
	}
	if g.pops != 0 || g.drops != 0 {
		t.Fatalf("pops=%d drops=%d, want 0/0", g.pops, g.drops)
	}
}

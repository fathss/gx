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

func (fake *fakeSyncGit) StashPush(label string) error {
	if fake.pushErr != nil {
		return fake.pushErr
	}
	fake.pushes = append(fake.pushes, label)
	fake.hasGX = true
	return nil
}

func (fake *fakeSyncGit) StashPop() error {
	if fake.popErr != nil {
		return fake.popErr
	}
	fake.pops++
	fake.hasGX = false
	return nil
}

func (fake *fakeSyncGit) DropGXStash() error {
	fake.drops++
	return fake.dropErr
}

func (fake *fakeSyncGit) IsClean() bool { return fake.clean }

func (fake *fakeSyncGit) HasGXStash() bool { return fake.hasGX }

func (fake *fakeSyncGit) Warnf(msg, hint string) {
	fake.warns = append(fake.warns, [2]string{msg, hint})
}

// Seam test 1: begin acquires a session by stashing a dirty tree.
func TestBeginDirtysPushesLabeledStash(t *testing.T) {
	fakeGit := &fakeSyncGit{clean: false}

	session, err := begin(fakeGit, "feature/login")
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if !session.acquired {
		t.Fatal("session not acquired")
	}
	if len(fakeGit.pushes) != 1 {
		t.Fatalf("pushes = %d, want 1", len(fakeGit.pushes))
	}
	if !strings.HasPrefix(fakeGit.pushes[0], "gx-sync/feature/login/") {
		t.Fatalf("label = %q, want gx-sync/feature/login/ prefix", fakeGit.pushes[0])
	}
	if fakeGit.pops != 0 || fakeGit.drops != 0 {
		t.Fatalf("begin must not settle: pops=%d drops=%d", fakeGit.pops, fakeGit.drops)
	}
}

// Seam test 1b: a clean tree begins with nothing to acquire.
func TestBeginCleanAcquiresNothing(t *testing.T) {
	fakeGit := &fakeSyncGit{clean: true}

	session, err := begin(fakeGit, "main")
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if session.acquired {
		t.Fatal("clean tree must not acquire")
	}
	if len(fakeGit.pushes) != 0 {
		t.Fatalf("pushes = %d, want 0", len(fakeGit.pushes))
	}
}

// Seam test 2: an act-phase error settles as restore — pop runs, the
// original error is what the caller sees.
func TestRunErrorRestoresBeforeReturning(t *testing.T) {
	fakeGit := &fakeSyncGit{clean: false}
	session, err := begin(fakeGit, "feature")
	if err != nil {
		t.Fatalf("begin: %v", err)
	}

	sentinel := &cli.Error{Message: "Failed to fetch remote."}
	got := session.run(func() (settle, error) { return settleRestore, sentinel })

	if got != sentinel {
		t.Fatalf("run = %v, want original error", got)
	}
	if fakeGit.pops != 1 {
		t.Fatalf("pops = %d, want 1", fakeGit.pops)
	}
	if fakeGit.drops != 0 {
		t.Fatalf("drops = %d, restore must not orphan-clean", fakeGit.drops)
	}
}

// Seam test 3: a conflict pause keeps the stash — that is what --continue
// and --abort resume later.
func TestRunPauseKeepsStash(t *testing.T) {
	fakeGit := &fakeSyncGit{clean: false}
	session, err := begin(fakeGit, "feature")
	if err != nil {
		t.Fatalf("begin: %v", err)
	}

	pauseErr := &cli.Error{Message: "Rebase stopped because of conflicts."}
	got := session.run(func() (settle, error) { return settlePause, pauseErr })

	if got != pauseErr {
		t.Fatalf("run = %v, want pause error", got)
	}
	if fakeGit.pops != 0 {
		t.Fatalf("pops = %d, want 0", fakeGit.pops)
	}
	if fakeGit.drops != 0 {
		t.Fatalf("drops = %d, want 0", fakeGit.drops)
	}
	if !fakeGit.hasGX {
		t.Fatal("pause must keep the gx stash")
	}
}

// Seam test 4: commit pops the stash and cleans up gx-sync/ entries, both
// for a session that acquired its own stash and one resumed from disk.
func TestRunCommitPopsAndDrops(t *testing.T) {
	fakeGit := &fakeSyncGit{clean: false}
	session, err := begin(fakeGit, "feature")
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if got := session.run(func() (settle, error) { return settleCommit, nil }); got != nil {
		t.Fatalf("run = %v, want nil", got)
	}
	if fakeGit.pops != 1 || fakeGit.drops != 1 {
		t.Fatalf("fresh commit: pops=%d drops=%d, want 1/1", fakeGit.pops, fakeGit.drops)
	}

	resumedGit := &fakeSyncGit{clean: true, hasGX: true}
	if got := resume(resumedGit).run(func() (settle, error) { return settleCommit, nil }); got != nil {
		t.Fatalf("resumed run = %v, want nil", got)
	}
	if resumedGit.pops != 1 || resumedGit.drops != 1 {
		t.Fatalf("resumed commit: pops=%d drops=%d, want 1/1", resumedGit.pops, resumedGit.drops)
	}
}

// Seam test 5: a pop conflict surfaces the standard settle copy, once.
func TestSettleRestoreConflictReturnsSettleCopy(t *testing.T) {
	fakeGit := &fakeSyncGit{clean: false, popErr: errStub}
	session, err := begin(fakeGit, "feature")
	if err != nil {
		t.Fatalf("begin: %v", err)
	}

	got := session.settle(settleRestore)
	conflictErr, ok := got.(*cli.Error)
	if !ok {
		t.Fatalf("settle = %#v, want *cli.Error", got)
	}
	if !strings.Contains(conflictErr.Message, "conflicts detected") {
		t.Fatalf("message = %q, want conflict copy", conflictErr.Message)
	}
	if fakeGit.drops != 0 {
		t.Fatalf("failed pop must not drop: drops=%d", fakeGit.drops)
	}
}

// Seam test 5b: when restore fails during an error-path settle, the caller
// still sees the original act-phase error and the conflict is reported as a
// warning instead of masking it.
func TestRunRestoreFailurePreservesOriginalError(t *testing.T) {
	fakeGit := &fakeSyncGit{clean: false, popErr: errStub}
	session, err := begin(fakeGit, "feature")
	if err != nil {
		t.Fatalf("begin: %v", err)
	}

	original := &cli.Error{Message: "Failed to fetch remote."}
	got := session.run(func() (settle, error) { return settleRestore, original })

	if got != original {
		t.Fatalf("run = %v, want original error", got)
	}
	if len(fakeGit.warns) != 1 || !strings.Contains(fakeGit.warns[0][0], "conflicts detected") {
		t.Fatalf("warns = %v, want one conflict warning", fakeGit.warns)
	}
}

// Seam test 6: resuming when no gx stash exists adopts nothing — the
// session pops nothing, and commit still attempts orphan cleanup.
func TestResumeWithoutStashPopsNothing(t *testing.T) {
	fakeGit := &fakeSyncGit{clean: true, hasGX: false}
	session := resume(fakeGit)

	if session.acquired {
		t.Fatal("resume without stash must not acquire")
	}
	if got := session.run(func() (settle, error) { return settleCommit, nil }); got != nil {
		t.Fatalf("run = %v, want nil", got)
	}
	if fakeGit.pops != 0 {
		t.Fatalf("pops = %d, want 0", fakeGit.pops)
	}
	if fakeGit.drops != 1 {
		t.Fatalf("drops = %d, want orphan cleanup even without stash", fakeGit.drops)
	}
}

// Restore settle on a session with nothing acquired is a no-op.
func TestSettleRestoreWithoutAcquireIsNoop(t *testing.T) {
	fakeGit := &fakeSyncGit{clean: true}
	session := resume(fakeGit)

	if got := session.settle(settleRestore); got != nil {
		t.Fatalf("settle = %v, want nil", got)
	}
	if fakeGit.pops != 0 || fakeGit.drops != 0 {
		t.Fatalf("pops=%d drops=%d, want 0/0", fakeGit.pops, fakeGit.drops)
	}
}

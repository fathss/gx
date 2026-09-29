package workflow

import (
	"errors"
	"strings"
	"testing"

	"github.com/fathss/gx/internal/cli"
	"github.com/fathss/gx/internal/config"
	"github.com/fathss/gx/internal/runner"
)

// shipFake returns a Fake pre-wired for Ship to reach the push step on the
// fast-forward path: no paused rebase/merge, on branch feat/login whose
// remote-tracking ref exists and is 1 commit behind, fetch succeeds.
func shipFake() *runner.Fake {
	return &runner.Fake{
		OutputResults: map[string]runner.OutputResult{
			"rev-parse --git-dir": {Out: ".git"},
			// Must fail: a nil error would read as "merge in progress".
			"rev-parse -q --verify MERGE_HEAD": {Err: errors.New("fatal: Needed a single revision")},
			"branch --show-current":            {Out: "feat/login"},
			// Remote-tracking ref exists → Tracking = true.
			"rev-parse --verify origin/feat/login":                   {Out: "refs/remotes/origin/feat/login"},
			"rev-list --count --left-right origin/feat/login...HEAD": {Out: "0\t1"},
		},
	}
}

func shipConfig() *config.Config {
	return &config.Config{
		Remote:            "origin",
		DefaultBranch:     "main",
		ProtectedBranches: []string{"main", "master"},
	}
}

func runShip(t *testing.T, fake *runner.Fake) error {
	t.Helper()
	return Ship(fake, shipConfig(), false, true)
}

func assertShipError(t *testing.T, err error, wantMessage, wantHintFragment string) *cli.Error {
	t.Helper()
	if err == nil {
		t.Fatal("expected Ship to fail")
	}
	var cliErr *cli.Error
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *cli.Error, got %T: %v", err, err)
	}
	if cliErr.Message != wantMessage {
		t.Errorf("Message = %q, want %q", cliErr.Message, wantMessage)
	}
	if !strings.Contains(cliErr.Hint, wantHintFragment) {
		t.Errorf("Hint = %q, want fragment %q", cliErr.Hint, wantHintFragment)
	}
	return cliErr
}

func TestShipPushRejectionKeepsSyncHintWhenNonFastForward(t *testing.T) {
	fake := shipFake()
	fake.RunResults = map[string]error{
		"push origin feat/login": &runner.CommandError{
			Args:     []string{"push", "origin", "feat/login"},
			ExitCode: 1,
			Stderr:   "! [rejected] feat/login -> feat/login (non-fast-forward)",
		},
	}

	cliErr := assertShipError(t, runShip(t, fake), "Push rejected by remote.", "run gx sync and retry")
	if !strings.Contains(cliErr.Hint, "Someone may have pushed") {
		t.Errorf("evidenced hint should keep the sync advice, got %q", cliErr.Hint)
	}
}

func TestShipPushRejectionFallsBackWhenStderrLacksEvidence(t *testing.T) {
	fake := shipFake()
	fake.RunResults = map[string]error{
		"push origin feat/login": &runner.CommandError{
			Args:     []string{"push", "origin", "feat/login"},
			ExitCode: 1,
			Stderr:   "remote: Permission denied\nfatal: could not read Username",
		},
	}

	cliErr := assertShipError(t, runShip(t, fake), "Push rejected by remote.", "Check the error above")
	if strings.Contains(cliErr.Hint, "may have pushed") {
		t.Errorf("hint invented a cause without evidence: %q", cliErr.Hint)
	}
}

func TestShipPushRejectionPlainErrorUsesGenericHint(t *testing.T) {
	fake := shipFake()
	fake.RunResults = map[string]error{
		"push origin feat/login": errors.New("exit status 1"),
	}

	cliErr := assertShipError(t, runShip(t, fake), "Push rejected by remote.", "Check the error above")
	if strings.Contains(cliErr.Hint, "may have pushed") {
		t.Errorf("hint invented a cause from a non-CommandError: %q", cliErr.Hint)
	}
}

func TestShipFetchFailureHintDoesNotInventCause(t *testing.T) {
	fake := shipFake()
	fake.RunResults = map[string]error{
		"fetch origin": &runner.CommandError{
			Args:     []string{"fetch", "origin"},
			ExitCode: 128,
			Stderr:   "fatal: could not read Username for 'https://github.com': terminal prompts disabled",
		},
	}

	cliErr := assertShipError(t, runShip(t, fake),
		`Failed to fetch branch "feat/login" from "origin".`,
		"verify the remote with `git remote -v`")
	if strings.Contains(cliErr.Hint, "network") || strings.Contains(cliErr.Hint, "Network") {
		t.Errorf("fetch hint guessed a cause: %q", cliErr.Hint)
	}
}

// A branch whose remote-tracking ref does not exist is a first push: ship
// still fetches (refs must be fresh before deciding) and then sets upstream.
func TestShipFirstPushFetchesThenSetsUpstreamWhenTrackingRefMissing(t *testing.T) {
	fake := shipFake()
	// Tracking ref missing → CheckDivergence returns Tracking=false.
	fake.OutputResults["rev-parse --verify origin/feat/login"] = runner.OutputResult{
		Err: errors.New("fatal: ambiguous argument 'origin/feat/login'"),
	}

	if err := runShip(t, fake); err != nil {
		t.Fatalf("Ship failed: %v", err)
	}

	var sawFetch, sawSetUpstream bool
	for _, call := range fake.Calls {
		key := strings.Join(call, " ")
		if key == "fetch origin" {
			sawFetch = true
		}
		if key == "push -u origin feat/login" {
			sawSetUpstream = true
		}
	}
	if !sawFetch {
		t.Error("first push should fetch before deciding (fresh refs)")
	}
	if !sawSetUpstream {
		t.Error("first push should use push -u")
	}
}

// A real git failure during the divergence check must surface as an error,
// never masquerade as "no upstream" (the old -1,-1 sentinel lie).
func TestShipDivergenceCheckFailureIsHonest(t *testing.T) {
	fake := shipFake()
	fake.OutputResults["rev-list --count --left-right origin/feat/login...HEAD"] = runner.OutputResult{
		Err: &runner.CommandError{
			Args:     []string{"rev-list", "--count", "--left-right", "origin/feat/login...HEAD"},
			ExitCode: 128,
			Stderr:   "fatal: not a valid object name",
		},
	}

	assertShipError(t, runShip(t, fake), "Failed to check divergence status.", "")
}

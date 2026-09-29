package git

import (
	"errors"
	"testing"

	"github.com/fathss/gx/internal/runner"
)

func TestCheckDivergenceCountsAheadBehind(t *testing.T) {
	fake := &runner.Fake{
		OutputResults: map[string]runner.OutputResult{
			"rev-parse --verify origin/feat/x":                   {Out: "refs/remotes/origin/feat/x"},
			"rev-list --count --left-right origin/feat/x...HEAD": {Out: "2\t5"},
		},
	}
	div, err := CheckDivergence(fake, "origin", "feat/x")
	if err != nil {
		t.Fatalf("CheckDivergence: %v", err)
	}
	if !div.Tracking {
		t.Error("Tracking = false, want true")
	}
	if div.Ahead != 5 || div.Behind != 2 {
		t.Errorf("got ahead=%d behind=%d, want ahead=5 behind=2", div.Ahead, div.Behind)
	}
}

// A missing remote-tracking ref is a normal state, not an error — and the
// rev-list query must be skipped entirely (no counting against a ref that
// does not exist).
func TestCheckDivergenceMissingTrackingRefIsNotAnError(t *testing.T) {
	fake := &runner.Fake{
		OutputResults: map[string]runner.OutputResult{
			"rev-parse --verify origin/new-branch": {Err: errors.New("fatal: ambiguous argument")},
		},
	}
	div, err := CheckDivergence(fake, "origin", "new-branch")
	if err != nil {
		t.Fatalf("CheckDivergence: %v", err)
	}
	if div.Tracking || div.Ahead != 0 || div.Behind != 0 {
		t.Errorf("got %+v, want zero Divergence", div)
	}
	for _, call := range fake.Calls {
		if call[0] == "rev-list" {
			t.Error("rev-list must not run when the tracking ref is missing")
		}
	}
}

// Real git failures must surface as errors — never as Tracking=false (the
// old -1,-1 sentinel could not tell these apart).
func TestCheckDivergencePropagatesQueryFailure(t *testing.T) {
	fake := &runner.Fake{
		OutputResults: map[string]runner.OutputResult{
			"rev-parse --verify origin/feat/x": {Out: "refs/remotes/origin/feat/x"},
			"rev-list --count --left-right origin/feat/x...HEAD": {
				Err: &runner.CommandError{
					Args:     []string{"rev-list"},
					ExitCode: 128,
					Stderr:   "fatal: not a valid object name",
				},
			},
		},
	}
	if _, err := CheckDivergence(fake, "origin", "feat/x"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCheckDivergenceRejectsUnparseableCounts(t *testing.T) {
	fake := &runner.Fake{
		OutputResults: map[string]runner.OutputResult{
			"rev-parse --verify origin/feat/x":                   {Out: "refs/remotes/origin/feat/x"},
			"rev-list --count --left-right origin/feat/x...HEAD": {Out: "garbage"},
		},
	}
	if _, err := CheckDivergence(fake, "origin", "feat/x"); err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

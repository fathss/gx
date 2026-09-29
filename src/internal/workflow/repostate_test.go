package workflow

import (
	"strings"
	"testing"

	"github.com/fathss/gx/internal/cli"
)

func TestClassifyCoversAllInputs(t *testing.T) {
	testCases := []struct {
		rebase, merge, gxStash bool
		want                   qVerdict
	}{
		{false, false, false, q1Idle},
		{false, false, true, q2OrphanStash},
		{true, false, false, q3ManualRebase},
		{true, false, true, q4GXRebase},
		{false, true, false, q5ManualMerge},
		{false, true, true, q6GXMerge},
		// Impossible rebase&&merge inputs: rebase precedence (historical order)
		{true, true, false, q3ManualRebase},
		{true, true, true, q4GXRebase},
	}
	for _, testCase := range testCases {
		got := classify(testCase.rebase, testCase.merge, testCase.gxStash)
		if got != testCase.want {
			t.Errorf("classify(%t,%t,%t) = %v, want %v",
				testCase.rebase, testCase.merge, testCase.gxStash, got, testCase.want)
		}
	}
}

func TestRequireNoInProgress(t *testing.T) {
	testCases := []struct {
		verdict     qVerdict
		wantErr     bool
		messagePart string
	}{
		{q1Idle, false, ""},
		{q2OrphanStash, false, ""},
		{q3ManualRebase, true, "rebase"},
		{q4GXRebase, true, "rebase"},
		{q5ManualMerge, true, "merge"},
		{q6GXMerge, true, "merge"},
	}
	for _, testCase := range testCases {
		err := requireNoInProgress(testCase.verdict)
		if !testCase.wantErr {
			if err != nil {
				t.Errorf("requireNoInProgress(%v) = %v, want nil", testCase.verdict, err)
			}
			continue
		}
		conflictErr, ok := err.(*cli.Error)
		if !ok {
			t.Errorf("requireNoInProgress(%v) = %#v, want *cli.Error", testCase.verdict, err)
			continue
		}
		if !strings.Contains(conflictErr.Message, testCase.messagePart) {
			t.Errorf("requireNoInProgress(%v) message = %q, want %q",
				testCase.verdict, conflictErr.Message, testCase.messagePart)
		}
		if conflictErr.Hint == "" {
			t.Errorf("requireNoInProgress(%v) has empty hint", testCase.verdict)
		}
	}
}

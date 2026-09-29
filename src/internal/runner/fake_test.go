package runner

import (
	"errors"
	"strings"
	"testing"
)

func TestFakeCannedResultsAndRecording(t *testing.T) {
	fake := &Fake{
		RunResults: map[string]error{
			"push origin feat": errors.New("push failed"),
		},
		OutputResults: map[string]OutputResult{
			"branch --show-current": {Out: "feat"},
		},
	}

	if err := fake.Run("push", "origin", "feat"); err == nil {
		t.Error("expected canned run error")
	}
	if err := fake.Run("fetch", "origin"); err != nil {
		t.Errorf("unknown command should default to success, got %v", err)
	}
	branch, err := fake.Output("branch", "--show-current")
	if err != nil || branch != "feat" {
		t.Errorf("Output = %q, %v; want %q, nil", branch, err, "feat")
	}
	if out, err := fake.Output("unknown", "args"); err != nil || out != "" {
		t.Errorf("unknown Output = %q, %v; want empty, nil", out, err)
	}

	fake.Warnf("something happened", "do this")
	if len(fake.Warnings) != 1 || fake.Warnings[0] != "something happened" {
		t.Errorf("Warnings = %q", fake.Warnings)
	}
	if len(fake.Calls) != 4 {
		t.Errorf("Calls recorded %d entries, want 4: %v", len(fake.Calls), fake.Calls)
	}
	if joined := strings.Join(fake.Calls[2], " "); joined != "branch --show-current" {
		t.Errorf("Calls[2] = %q", joined)
	}
}

package git

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/fathss/gx/internal/runner"
)

func TestMergedBranchesStripsCurrentMarker(t *testing.T) {
	fake := &runner.Fake{
		OutputResults: map[string]runner.OutputResult{
			"branch --merged develop": {Out: "  feature/a\n* feat/current\n  fix/b\n"},
		},
	}
	got, err := MergedBranches(fake, "develop")
	if err != nil {
		t.Fatalf("MergedBranches: %v", err)
	}
	want := []string{"feature/a", "feat/current", "fix/b"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestMergedRemoteBranchesNormalizesAtBoundary(t *testing.T) {
	fake := &runner.Fake{
		OutputResults: map[string]runner.OutputResult{
			"branch -r --merged origin/develop": {Out: strings.Join([]string{
				"  origin/feature/a",
				"  origin/HEAD -> origin/develop",
				"  origin/develop",
				"  upstream/feature/foreign",
				"  origin/feat/c",
			}, "\n")},
		},
	}
	got, err := MergedRemoteBranches(fake, "origin", "develop")
	if err != nil {
		t.Fatalf("MergedRemoteBranches: %v", err)
	}
	// Arrow lines and foreign-remote refs are format noise → dropped below
	// the seam. The default branch survives here: dropping it is policy,
	// which the workflow's filterBranches applies to bare names.
	want := []string{"feature/a", "develop", "feat/c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestMergedRemoteBranchesQualifiesBase(t *testing.T) {
	fake := &runner.Fake{
		OutputResults: map[string]runner.OutputResult{
			"branch -r --merged origin/develop": {Out: "  origin/feature/a"},
		},
	}
	if _, err := MergedRemoteBranches(fake, "origin", "origin/develop"); err != nil {
		t.Fatalf("MergedRemoteBranches: %v", err)
	}
	// Already-qualified base must not be double-qualified; assert via the
	// recorded call.
	want := []string{"branch", "-r", "--merged", "origin/develop"}
	if len(fake.Calls) != 1 || !reflect.DeepEqual(fake.Calls[0], want) {
		t.Errorf("calls = %v, want [%v]", fake.Calls, want)
	}
}

func TestMergedRemoteBranchesPropagatesFailure(t *testing.T) {
	fake := &runner.Fake{
		OutputErr: errors.New("boom"),
	}
	if _, err := MergedRemoteBranches(fake, "origin", "develop"); err == nil {
		t.Error("expected error to propagate")
	}
}

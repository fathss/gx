package workflow

import (
	"reflect"
	"testing"

	"github.com/fathss/gx/internal/config"
)

func cleanFilterConfig() *config.Config {
	return &config.Config{
		Remote:            "origin",
		DefaultBranch:     "develop",
		ProtectedBranches: []string{"main", "master"},
	}
}

func TestFilterBranchesDropsDefaultAndProtected(t *testing.T) {
	got := filterBranches(
		[]string{"develop", "main", "feature/a", "fix/b", ""},
		cleanFilterConfig(),
	)
	want := []string{"feature/a", "fix/b"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestFilterBranchesRemoteNamesMatchProtectedPolicy(t *testing.T) {
	// Remote candidates arrive as bare names ("main"), so the same policy
	// applies without any remote-prefix logic in the workflow.
	got := filterBranches(
		[]string{"main", "feature/a", "develop"},
		cleanFilterConfig(),
	)
	want := []string{"feature/a"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestFilterLocalBranchesDropsCurrent(t *testing.T) {
	got := filterLocalBranches(
		[]string{"feat/current", "feature/a", "develop"},
		cleanFilterConfig(),
		"feat/current",
	)
	want := []string{"feature/a"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// Remote-tracking candidates deliberately keep the checked-out branch's
// counterpart: origin/<current> stays prunable while the local branch is in
// use. filterBranches (the remote path) must not exclude it.
func TestFilterBranchesKeepsCurrentForRemoteTracking(t *testing.T) {
	got := filterBranches(
		[]string{"feat/current", "feature/a"},
		cleanFilterConfig(),
	)
	want := []string{"feat/current", "feature/a"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

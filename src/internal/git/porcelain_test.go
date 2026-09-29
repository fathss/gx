package git

import (
	"reflect"
	"strings"
	"testing"

	"github.com/fathss/gx/internal/runner"
)

func TestParsePorcelain(t *testing.T) {
	tests := []struct {
		name          string
		in            string
		staged        map[string][]string
		unstaged      []string
		untracked     []string
		conflicted    []string
		modifiedFiles []string
	}{
		{
			name:   "empty output yields empty buckets with initialized map",
			in:     "",
			staged: map[string][]string{},
		},
		{
			name:          "leading space column survives on first line",
			in:            " M worktree-change.txt\n?? notes.md",
			unstaged:      []string{"worktree-change.txt"},
			untracked:     []string{"notes.md"},
			modifiedFiles: []string{"worktree-change.txt"},
			staged:        map[string][]string{},
		},
		{
			name:          "staged modification",
			in:            "M  staged.txt",
			staged:        map[string][]string{"M": {"staged.txt"}},
			modifiedFiles: []string{"staged.txt"},
		},
		{
			name:          "staged and unstaged on same file counts once in modified",
			in:            "MM both.txt",
			staged:        map[string][]string{"M": {"both.txt"}},
			unstaged:      []string{"both.txt"},
			modifiedFiles: []string{"both.txt"},
		},
		{
			name:          "staged new file then deleted in worktree",
			in:            "AD add-then-gone.txt",
			staged:        map[string][]string{"A": {"add-then-gone.txt"}},
			unstaged:      []string{"add-then-gone.txt"},
			modifiedFiles: []string{"add-then-gone.txt"},
		},
		{
			name:          "staged deletion",
			in:            "D  gone.txt",
			staged:        map[string][]string{"D": {"gone.txt"}},
			modifiedFiles: []string{"gone.txt"},
		},
		{
			name:          "unstaged deletion",
			in:            " D gone.txt",
			unstaged:      []string{"gone.txt"},
			modifiedFiles: []string{"gone.txt"},
		},
		{
			name:          "staged rename contributes destination only",
			in:            "R  old-name.txt -> new-name.txt",
			staged:        map[string][]string{"R": {"new-name.txt"}},
			modifiedFiles: []string{"new-name.txt"},
		},
		{
			name:          "unmerged both-modified stays out of staged and unstaged",
			in:            "UU conflict.txt",
			conflicted:    []string{"conflict.txt"},
			modifiedFiles: []string{"conflict.txt"},
			staged:        map[string][]string{},
		},
		{
			name:          "both-added conflict is not a staged add",
			in:            "AA both-added.txt",
			conflicted:    []string{"both-added.txt"},
			modifiedFiles: []string{"both-added.txt"},
			staged:        map[string][]string{},
		},
		{
			name:          "both-deleted conflict",
			in:            "DD both-deleted.txt",
			conflicted:    []string{"both-deleted.txt"},
			modifiedFiles: []string{"both-deleted.txt"},
			staged:        map[string][]string{},
		},
		{
			name: "kitchen sink keeps porcelain order",
			in: strings.Join([]string{
				"M  staged.txt",
				" M worktree.txt",
				"MM both.txt",
				"?? untracked.txt",
				"UU conflict.txt",
				"R  old.txt -> new.txt",
			}, "\n"),
			staged: map[string][]string{
				"M": {"staged.txt", "both.txt"},
				"R": {"new.txt"},
			},
			unstaged:      []string{"worktree.txt", "both.txt"},
			untracked:     []string{"untracked.txt"},
			conflicted:    []string{"conflict.txt"},
			modifiedFiles: []string{"staged.txt", "worktree.txt", "both.txt", "conflict.txt", "new.txt"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := parsePorcelain(testCase.in)
			expectedStaged := testCase.staged
			if expectedStaged == nil {
				expectedStaged = map[string][]string{}
			}
			if !reflect.DeepEqual(got.StagedFiles, expectedStaged) {
				t.Errorf("StagedFiles = %v, want %v", got.StagedFiles, expectedStaged)
			}
			if !reflect.DeepEqual(got.UnstagedFiles, testCase.unstaged) {
				t.Errorf("UnstagedFiles = %v, want %v", got.UnstagedFiles, testCase.unstaged)
			}
			if !reflect.DeepEqual(got.UntrackedFiles, testCase.untracked) {
				t.Errorf("UntrackedFiles = %v, want %v", got.UntrackedFiles, testCase.untracked)
			}
			if !reflect.DeepEqual(got.ConflictedFiles, testCase.conflicted) {
				t.Errorf("ConflictedFiles = %v, want %v", got.ConflictedFiles, testCase.conflicted)
			}
			if !reflect.DeepEqual(got.ModifiedFiles, testCase.modifiedFiles) {
				t.Errorf("ModifiedFiles = %v, want %v", got.ModifiedFiles, testCase.modifiedFiles)
			}
		})
	}
}

// TestGetParsedStatusSinglePorcelainQuery pins the 4→1 subprocess collapse
// and both views reading the same parse.
func TestGetParsedStatusSinglePorcelainQuery(t *testing.T) {
	fake := &runner.Fake{
		OutputResults: map[string]runner.OutputResult{
			"status --porcelain --untracked-files=all": {
				Out: "M  staged.txt\n?? untracked.txt\nUU conflict.txt",
			},
		},
	}

	ps, err := GetParsedStatus(fake)
	if err != nil {
		t.Fatalf("GetParsedStatus: %v", err)
	}
	if len(fake.Calls) != 1 {
		t.Fatalf("expected exactly 1 git call, got %d: %v", len(fake.Calls), fake.Calls)
	}
	if got := strings.Join(fake.Calls[0], " "); got != "status --porcelain --untracked-files=all" {
		t.Errorf("call = %q", got)
	}
	if !reflect.DeepEqual(ps.StagedFiles["M"], []string{"staged.txt"}) {
		t.Errorf("StagedFiles[M] = %v", ps.StagedFiles["M"])
	}
	if !reflect.DeepEqual(ps.UntrackedFiles, []string{"untracked.txt"}) {
		t.Errorf("UntrackedFiles = %v", ps.UntrackedFiles)
	}
	if !reflect.DeepEqual(ps.ConflictedFiles, []string{"conflict.txt"}) {
		t.Errorf("ConflictedFiles = %v", ps.ConflictedFiles)
	}
}

func TestStatusSharesPorcelainParse(t *testing.T) {
	fake := &runner.Fake{
		OutputResults: map[string]runner.OutputResult{
			"status --porcelain --untracked-files=all": {
				Out: "M  staged.txt\n M worktree.txt\n?? notes.md",
			},
		},
	}

	modified, untracked, err := Status(fake)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !reflect.DeepEqual(modified, []string{"staged.txt", "worktree.txt"}) {
		t.Errorf("modified = %v", modified)
	}
	if !reflect.DeepEqual(untracked, []string{"notes.md"}) {
		t.Errorf("untracked = %v", untracked)
	}
	if len(fake.Calls) != 1 {
		t.Errorf("expected 1 call, got %d", len(fake.Calls))
	}
}

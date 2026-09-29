package git

import (
	"errors"
	"testing"

	"github.com/fathss/gx/internal/runner"
)

func TestIsNonFastForward(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "command error with non-fast-forward marker",
			err: &runner.CommandError{Args: []string{"push", "origin", "feat"}, ExitCode: 1,
				Stderr: "! [rejected]  feat -> feat (non-fast-forward)\nerror: failed to push some refs to 'origin'"},
			want: true,
		},
		{
			name: "command error with unrelated stderr",
			err: &runner.CommandError{Args: []string{"push", "origin", "feat"}, ExitCode: 1,
				Stderr: "remote: Permission denied\nfatal: could not read Username"},
			want: false,
		},
		{
			name: "command error without stderr",
			err:  &runner.CommandError{Args: []string{"push", "origin", "feat"}, ExitCode: 1},
			want: false,
		},
		{
			name: "plain error is not evidence",
			err:  errors.New("exit status 1"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if got := IsNonFastForward(testCase.err); got != testCase.want {
				t.Errorf("IsNonFastForward(%v) = %v, want %v", testCase.err, got, testCase.want)
			}
		})
	}
}

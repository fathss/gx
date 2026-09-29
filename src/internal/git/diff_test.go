package git

import (
	"errors"
	"testing"

	"github.com/fathss/gx/internal/runner"
)

func TestHasStagedChangesExitCodes(t *testing.T) {
	tests := []struct {
		name    string
		fake    *runner.Fake
		want    bool
		wantErr bool
	}{
		{
			name: "clean index (exit 0) → false",
			fake: &runner.Fake{},
			want: false,
		},
		{
			name: "differences (exit 1) → true",
			fake: &runner.Fake{
				OutputErr: &runner.CommandError{
					Args:     []string{"diff", "--cached", "--quiet"},
					ExitCode: 1,
				},
			},
			want: true,
		},
		{
			name: "broken repository (exit 128) → error, not a staged verdict",
			fake: &runner.Fake{
				OutputErr: &runner.CommandError{
					Args:     []string{"diff", "--cached", "--quiet"},
					ExitCode: 128,
					Stderr:   "fatal: not a git repository",
				},
			},
			wantErr: true,
		},
		{
			name: "infrastructure failure → error",
			fake: &runner.Fake{
				OutputErr: errors.New("exec: git: executable file not found"),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HasStagedChanges(tt.fake)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

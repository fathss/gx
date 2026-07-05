package git

import "github.com/fathss/gx/internal/runner"

func IsRepository(run *runner.Runner) bool {
	_, err := run.Output("rev-parse", "--git-dir")
	return err == nil
}
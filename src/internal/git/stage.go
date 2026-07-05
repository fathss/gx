package git

import "github.com/fathss/gx/internal/runner"

// AddAll stages all changes (modified tracked + untracked) in the working tree.
// Equivalent to `git add -A`.
func AddAll(run *runner.Runner) error {
	return run.Run("add", "-A")
}

// Add stages the specified files. If no files are given, Add stages everything.
// Equivalent to `git add <files...>`.
func Add(run *runner.Runner, files ...string) error {
	args := append([]string{"add"}, files...)
	return run.Run(args...)
}

// AddPatch runs interactive hunk selection. If files are given, only hunks
// in those files are shown. Equivalent to `git add -p [<files>...]`.
func AddPatch(run *runner.Runner, files ...string) error {
	args := append([]string{"add", "-p"}, files...)
	return run.Run(args...)
}

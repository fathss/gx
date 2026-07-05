package runner

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// WarnFunc is a handler for non-fatal warnings.
// The cmd layer wires this to write to stderr with appropriate formatting.
type WarnFunc func(msg, hint string)

type Runner struct {
	Verbose bool
	WarnFn  WarnFunc
}

// Warn emits a non-fatal warning message via the configured handler.
func (r *Runner) Warn(msg string) {
	r.Warnf(msg, "")
}

// Warnf emits a non-fatal warning with a hint.
func (r *Runner) Warnf(msg, hint string) {
	if r.WarnFn != nil {
		r.WarnFn(msg, hint)
	}
}

func New(verbose bool) *Runner {
	return &Runner{Verbose: verbose}
}

func (r *Runner) Run(args ...string) error {
	fmt.Println("▸ git", strings.Join(args, " "))

	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// RunWithEnv is like Run but prepends extraEnv to the subprocess environment.
func (r *Runner) RunWithEnv(extraEnv []string, args ...string) error {
	fmt.Println("▸ git", strings.Join(args, " "))

	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(), extraEnv...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func (r *Runner) Output(args ...string) (string, error) {
	if r.Verbose {
		fmt.Println("▸ git", strings.Join(args, " "))
	}

	out, err := exec.Command("git", args...).Output()
	return strings.TrimSpace(string(out)), err
}

func (r *Runner) CombinedOutput(args ...string) (string, error) {
    if r.Verbose {
        fmt.Println("▸ git", strings.Join(args, " "))
    }

    out, err := exec.Command("git", args...).CombinedOutput()
    return strings.TrimSpace(string(out)), err
}
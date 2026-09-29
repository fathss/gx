package runner

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// WarnFunc is a handler for non-fatal warnings.
// The cmd layer wires this to write to stderr with appropriate formatting.
type WarnFunc func(msg, hint string)

// Executor is the seam through which the rest of gx runs git commands.
// The production implementation is *Runner; tests provide their own
// adapters (see Fake). Declaring the interface next to its implementation
// keeps both adapters in one place.
type Executor interface {
	Run(args ...string) error
	RunWithEnv(extraEnv []string, args ...string) error
	Output(args ...string) (string, error)
	CombinedOutput(args ...string) (string, error)
	Warn(msg string)
	Warnf(msg, hint string)
}

// CommandError reports a failed git command together with the stderr it
// produced. Workflows inspect it with errors.As to quote git's real cause
// instead of inventing one.
type CommandError struct {
	Args     []string
	ExitCode int
	Stderr   string // right-trimmed; for CombinedOutput it carries the combined stream
}

func (e *CommandError) Error() string {
	msg := fmt.Sprintf("git %s failed (exit %d)", strings.Join(e.Args, " "), e.ExitCode)
	if e.Stderr != "" {
		msg += ": " + strings.TrimSpace(e.Stderr)
	}
	return msg
}

type Runner struct {
	Verbose bool
	WarnFn  WarnFunc
}

var _ Executor = (*Runner)(nil)

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

// trimOutput strips trailing line breaks only. Leading whitespace is
// significant (git porcelain status columns) and must survive.
func trimOutput(s string) string {
	return strings.TrimRight(s, "\r\n")
}

// newCommandError wraps an exec failure in a CommandError when it came from
// a process that actually ran; infrastructure errors pass through unwrapped.
func newCommandError(args []string, err error, stderr string) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return &CommandError{
			Args:     append([]string(nil), args...),
			ExitCode: exitErr.ExitCode(),
			Stderr:   trimOutput(stderr),
		}
	}
	return err
}

func (r *Runner) Run(args ...string) error {
	fmt.Println("▸ git", strings.Join(args, " "))

	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout

	// Tee stderr: stream it live for the user, capture it for CommandError.
	var stderrBuffer bytes.Buffer
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuffer)

	if err := cmd.Run(); err != nil {
		return newCommandError(args, err, stderrBuffer.String())
	}
	return nil
}

// RunWithEnv is like Run but prepends extraEnv to the subprocess environment.
func (r *Runner) RunWithEnv(extraEnv []string, args ...string) error {
	fmt.Println("▸ git", strings.Join(args, " "))

	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(), extraEnv...)
	cmd.Stdout = os.Stdout

	var stderrBuffer bytes.Buffer
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuffer)

	if err := cmd.Run(); err != nil {
		return newCommandError(args, err, stderrBuffer.String())
	}
	return nil
}

func (r *Runner) Output(args ...string) (string, error) {
	if r.Verbose {
		fmt.Println("▸ git", strings.Join(args, " "))
	}

	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		// With cmd.Stderr nil, exec captures stderr into ExitError.Stderr.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return trimOutput(string(out)), newCommandError(args, err, string(exitErr.Stderr))
		}
		return trimOutput(string(out)), err
	}
	return trimOutput(string(out)), nil
}

func (r *Runner) CombinedOutput(args ...string) (string, error) {
	if r.Verbose {
		fmt.Println("▸ git", strings.Join(args, " "))
	}

	out, err := exec.Command("git", args...).CombinedOutput()
	trimmed := trimOutput(string(out))
	if err != nil {
		return trimmed, newCommandError(args, err, trimmed)
	}
	return trimmed, nil
}

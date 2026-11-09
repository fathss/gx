package runner

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/fathss/gx/internal/ui"
)

// WarnFunc is a handler for non-fatal warnings.
// The cmd layer wires this to write to stderr with appropriate formatting.
type WarnFunc func(msg, hint string)

// SuccessFunc is a handler for non-fatal success notifications.
type SuccessFunc func(msg string)

// InfoFunc is a handler for non-fatal informational messages.
type InfoFunc func(msg string)

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
	Success(msg string)
	Info(msg string)
	StartProgress(msg string)
	UpdateProgress(msg string)
	EndProgress(success bool, msg, hint string)
	State(msg string)
	StopSpinner()
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
	Verbose   bool
	WarnFn    WarnFunc
	SuccessFn SuccessFunc
	InfoFn    InfoFunc
	spinner   *ui.Spinner
	mu        sync.Mutex
}

var _ Executor = (*Runner)(nil)

// isInteractive reports whether interactive animations (like spinners) should be used.
func (r *Runner) isInteractive() bool {
	if r.Verbose {
		return false
	}
	if os.Getenv("GX_INTERACTIVE") == "1" {
		return true
	}
	return ui.IsTerminal(os.Stdout.Fd())
}

// State announces a new workflow state, printing the state message in dim text
// and ensuring a single spinner is active below it.
func (r *Runner) State(msg string) {
	if !r.isInteractive() {
		fmt.Println(ui.Dim(msg))
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.spinner != nil {
		r.spinner.Stop()
		r.spinner = nil
	}

	fmt.Println(ui.Dim(msg))

	r.spinner = ui.NewSpinner("", ui.WithTTY(true))
	r.spinner.Start()
}

// StopSpinner stops and clears the current spinner if one is active.
func (r *Runner) StopSpinner() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.spinner != nil {
		r.spinner.Stop()
		r.spinner = nil
	}
}

// StartProgress begins a progress indicator with the given message.
func (r *Runner) StartProgress(msg string) {
	r.State(msg)
}

// UpdateProgress updates the current progress message.
func (r *Runner) UpdateProgress(msg string) {
	r.State(msg)
}

// EndProgress terminates the current progress indicator with success or failure.
func (r *Runner) EndProgress(success bool, msg, hint string) {
	r.StopSpinner()
	if success {
		if msg != "" {
			r.Success(msg)
		}
	} else {
		if msg != "" {
			r.Warnf(msg, hint)
		}
	}
}

// Warn emits a non-fatal warning message via the configured handler.
func (r *Runner) Warn(msg string) {
	r.Warnf(msg, "")
}

// Warnf emits a non-fatal warning with a hint.
func (r *Runner) Warnf(msg, hint string) {
	r.StopSpinner()
	if r.WarnFn != nil {
		r.WarnFn(msg, hint)
	}
}

// Success emits a non-fatal success notification via the configured handler.
func (r *Runner) Success(msg string) {
	r.StopSpinner()
	if r.SuccessFn != nil {
		r.SuccessFn(msg)
	}
}

// Info emits a non-fatal informational message via the configured handler.
func (r *Runner) Info(msg string) {
	if r.InfoFn != nil {
		r.InfoFn(msg)
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

func (r *Runner) runCommand(cmd *exec.Cmd, args []string) error {
	if r.Verbose {
		r.StopSpinner()
		fmt.Println("▸ git", strings.Join(args, " "))
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout

		var stderrBuffer bytes.Buffer
		cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuffer)

		if err := cmd.Run(); err != nil {
			return newCommandError(args, err, stderrBuffer.String())
		}
		return nil
	}

	cmd.Stdin = os.Stdin

	// Commands like 'commit' produce commit summaries that must be visible on stdout
	isCommit := len(args) > 0 && args[0] == "commit"
	if isCommit {
		r.StopSpinner()
		cmd.Stdout = os.Stdout
		var stderrBuffer bytes.Buffer
		cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuffer)
		if err := cmd.Run(); err != nil {
			return newCommandError(args, err, stderrBuffer.String())
		}
		return nil
	}

	var stdoutBuffer, stderrBuffer bytes.Buffer
	cmd.Stdout = &stdoutBuffer
	cmd.Stderr = &stderrBuffer

	err := cmd.Run()
	if err != nil {
		r.StopSpinner()
		return newCommandError(args, err, stderrBuffer.String())
	}
	return nil
}

func (r *Runner) Run(args ...string) error {
	cmd := exec.Command("git", args...)
	return r.runCommand(cmd, args)
}

// RunWithEnv is like Run but prepends extraEnv to the subprocess environment.
func (r *Runner) RunWithEnv(extraEnv []string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(), extraEnv...)
	return r.runCommand(cmd, args)
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

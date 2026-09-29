package runner

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// initTestRepo creates a temp git repository with one committed file and
// chdirs into it, so the Runner's process-wide git calls hit the fixture.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test User")
	return dir
}

func TestOutputRightTrimPreservesLeadingSpace(t *testing.T) {
	dir := initTestRepo(t)
	file := dir + "/tracked-change.txt"
	if err := os.WriteFile(file, []byte("v1\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	seed := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("seed git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	seed("add", "tracked-change.txt")
	seed("commit", "-m", "seed")
	if err := os.WriteFile(file, []byte("v2\n"), 0o644); err != nil {
		t.Fatalf("modify file: %v", err)
	}

	out, err := New(false).Output("status", "--porcelain")
	if err != nil {
		t.Fatalf("Output: %v", err)
	}
	// Unstaged modification: first line is " M tracked-change.txt" — the
	// leading status column must survive right-trim.
	if !strings.HasPrefix(out, " ") {
		t.Errorf("leading space stripped: got %q", out)
	}
	if strings.HasSuffix(out, "\n") {
		t.Errorf("trailing newline not stripped: got %q", out)
	}
	if !strings.Contains(out, "tracked-change.txt") {
		t.Errorf("expected file in porcelain output, got %q", out)
	}
}

func TestOutputFailureCarriesStderr(t *testing.T) {
	initTestRepo(t)

	out, err := New(false).Output("rev-parse", "--verify", "refs/heads/gx-no-such-branch")
	if err == nil {
		t.Fatal("expected failure for missing ref")
	}
	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected *CommandError, got %T: %v", err, err)
	}
	if cmdErr.ExitCode == 0 {
		t.Errorf("expected non-zero exit code, got %d", cmdErr.ExitCode)
	}
	if strings.TrimSpace(cmdErr.Stderr) == "" {
		t.Error("expected captured stderr to be non-empty")
	}
	if want := []string{"rev-parse", "--verify", "refs/heads/gx-no-such-branch"}; strings.Join(cmdErr.Args, " ") != strings.Join(want, " ") {
		t.Errorf("Args = %q, want %q", cmdErr.Args, want)
	}
	if out != "" {
		t.Errorf("expected empty stdout, got %q", out)
	}
}

func TestRunFailureCarriesStderr(t *testing.T) {
	initTestRepo(t)

	err := New(false).Run("rev-parse", "--verify", "refs/heads/gx-no-such-branch")
	if err == nil {
		t.Fatal("expected failure for missing ref")
	}
	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected *CommandError, got %T: %v", err, err)
	}
	if strings.TrimSpace(cmdErr.Stderr) == "" {
		t.Error("expected tee'd stderr to be non-empty")
	}
	if !strings.Contains(cmdErr.Error(), "git rev-parse") {
		t.Errorf("Error() should name the command, got %q", cmdErr.Error())
	}
}

func TestOutputSuccessHasNoCommandError(t *testing.T) {
	initTestRepo(t)

	if _, err := New(false).Output("rev-parse", "--is-inside-work-tree"); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

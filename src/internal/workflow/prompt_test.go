package workflow

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/fathss/gx/internal/cli"
)

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errors.New("boom")
}

type mustNotRead struct{ t *testing.T }

func (r mustNotRead) Read([]byte) (int, error) {
	r.t.Error("reader was touched under autoYes")
	return 0, io.EOF
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = writer
	defer func() { os.Stderr = old }()
	fn()
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close reader: %v", err)
	}
	return string(out)
}

func TestConfirmAnswersFromReader(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"lower y", "y\n", true},
		{"upper Y", "Y\n", true},
		{"full yes", "yes\n", true},
		{"upper YES", "YES\n", true},
		{"padded y", " y \n", true},
		{"n", "n\n", false},
		{"full no", "no\n", false},
		{"empty line", "\n", false},
		{"immediate EOF", "", false},
		{"EOF with partial y", "y", true},
		{"EOF with partial n", "n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prompter := newPrompter(strings.NewReader(tc.input), false)
			var got bool
			var err error
			captureStderr(t, func() {
				got, err = prompter.Confirm("Stage it?")
			})
			if err != nil {
				t.Fatalf("Confirm: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestConfirmKeepsBufferedAnswersAcrossPrompts(t *testing.T) {
	prompter := newPrompter(strings.NewReader("y\nn\n"), false)
	var first, second bool
	var err error
	captureStderr(t, func() {
		first, err = prompter.Confirm("Stage it?")
	})
	if err != nil {
		t.Fatalf("first Confirm: %v", err)
	}
	if !first {
		t.Error("first answer: got false, want true")
	}
	captureStderr(t, func() {
		second, err = prompter.Confirm("Commit it?")
	})
	if err != nil {
		t.Fatalf("second Confirm: %v", err)
	}
	if second {
		t.Error("second answer: got true, want false")
	}
}

func TestConfirmReadFailureIsUserFacingError(t *testing.T) {
	prompter := newPrompter(failingReader{}, false)
	var confirmed bool
	var err error
	captureStderr(t, func() {
		confirmed, err = prompter.Confirm("Stage it?")
	})
	if confirmed {
		t.Errorf("got confirmed=true on read failure, want false")
	}
	if err == nil {
		t.Fatal("got nil error on read failure, want an error")
	}
	var cliErr *cli.Error
	if !errors.As(err, &cliErr) {
		t.Fatalf("got %v (%T), want *cli.Error", err, err)
	}
	if !strings.Contains(cliErr.Message, "Failed to read input") {
		t.Errorf("Message = %q, want it to mention Failed to read input", cliErr.Message)
	}
}

func TestConfirmAutoYesSkipsPromptAndRead(t *testing.T) {
	prompter := newPrompter(mustNotRead{t}, true)
	confirmed, err := prompter.Confirm("Stage it?")
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if !confirmed {
		t.Error("got false under autoYes, want true")
	}
}

func TestConfirmPromptPrintsToStderr(t *testing.T) {
	out := captureStderr(t, func() {
		prompter := newPrompter(strings.NewReader("n\n"), false)
		if _, err := prompter.Confirm("Stage it?"); err != nil {
			t.Errorf("Confirm: %v", err)
		}
	})
	if out != "Stage it? [y/N]: " {
		t.Errorf("stderr = %q, want %q", out, "Stage it? [y/N]: ")
	}
}

func TestConfirmThroughProcessPrompter(t *testing.T) {
	original := prompts
	prompts = newPrompter(strings.NewReader("y\n"), false)
	t.Cleanup(func() { prompts = original })

	var confirmed bool
	var err error
	captureStderr(t, func() {
		confirmed, err = prompts.Confirm("Stage it?")
	})
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if !confirmed {
		t.Error("got false, want true")
	}
}

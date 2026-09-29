package workflow

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fathss/gx/internal/cli"
)

// prompter seats one shared buffered reader over an input stream and owns
// the yes/no prompt policy. One bufio.Reader per process is load-bearing:
// a fresh reader per prompt would discard bytes it already buffered, so a
// second prompt would never see the user's remaining answers.
//
// Two adapters seat the same implementation: os.Stdin in production and a
// scripted reader in tests, which re-seat `prompts` directly.
type prompter struct {
	in      *bufio.Reader
	autoYes bool
}

// newPrompter seats a prompter on r. autoYes answers every question with
// yes without printing or reading (the --yes vocabulary).
func newPrompter(r io.Reader, autoYes bool) *prompter {
	return &prompter{in: bufio.NewReader(r), autoYes: autoYes}
}

// prompts is the process-wide prompter, seated on os.Stdin for the binary.
var prompts = newPrompter(os.Stdin, false)

// Confirm asks a yes/no question on the terminal and returns true only for
// y/yes (case-insensitive). n, N, an empty line, and EOF (piped or closed
// stdin) all mean no; EOF still carries whatever was typed before the
// stream closed. A read failure other than EOF is a user-facing error.
// The prompt prints to stderr so redirecting stdout keeps the dialogue on
// the terminal.
func (p *prompter) Confirm(prompt string) (bool, error) {
	if p.autoYes {
		return true, nil
	}
	fmt.Fprintf(os.Stderr, "%s [y/N]: ", prompt)
	line, err := p.in.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, &cli.Error{
			Message: fmt.Sprintf("Failed to read input: %s.", err),
		}
	}
	answer := strings.TrimSpace(line)
	return strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes"), nil
}

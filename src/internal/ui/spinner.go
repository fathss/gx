package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// ANSI terminal control codes
const (
	HideCursorCode = "\033[?25l"
	ShowCursorCode = "\033[?25h"
	ClearLineCode  = "\r\033[K"
)

// DefaultSlashFrames defines the standard ASCII slash spinner animation frames.
var DefaultSlashFrames = []string{"|", "/", "-", "\\"}

// SpinnerOption configures a Spinner instance.
type SpinnerOption func(*Spinner)

// WithWriter sets the output writer for the spinner.
func WithWriter(w io.Writer) SpinnerOption {
	return func(s *Spinner) {
		s.out = w
	}
}

// WithInterval sets the tick interval for the spinner animation.
func WithInterval(d time.Duration) SpinnerOption {
	return func(s *Spinner) {
		s.interval = d
	}
}

// WithTTY overrides the interactive terminal detection.
func WithTTY(isTTY bool) SpinnerOption {
	return func(s *Spinner) {
		s.isTTY = isTTY
	}
}

// WithFrames overrides the spinner animation frames.
func WithFrames(frames []string) SpinnerOption {
	return func(s *Spinner) {
		if len(frames) > 0 {
			s.frames = frames
		}
	}
}

// Spinner provides an ASCII slash loading spinner for terminal workflows.
type Spinner struct {
	mu       sync.Mutex
	active   bool
	message  string
	frames   []string
	frameIdx int
	interval time.Duration
	out      io.Writer
	isTTY    bool
	stopCh   chan struct{}
	doneCh   chan struct{}
}

// NewSpinner creates a new ASCII slash spinner with the given initial message.
func NewSpinner(message string, opts ...SpinnerOption) *Spinner {
	s := &Spinner{
		message:  message,
		frames:   DefaultSlashFrames,
		interval: 100 * time.Millisecond,
		out:      os.Stdout,
		isTTY:    IsTerminal(os.Stdout.Fd()),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Start activates the spinner and begins the animation loop in a background goroutine.
func (s *Spinner) Start() {
	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		return
	}

	s.active = true
	s.frameIdx = 0
	s.stopCh = make(chan struct{})
	s.doneCh = make(chan struct{})

	if s.isTTY {
		fmt.Fprint(s.out, HideCursorCode)
		s.renderLine()
	}

	s.mu.Unlock()

	go s.loop()
}

// loop runs the animation timer.
func (s *Spinner) loop() {
	defer close(s.doneCh)

	if !s.isTTY {
		// Non-interactive: wait until stopped without emitting animations.
		<-s.stopCh
		return
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.mu.Lock()
			if !s.active {
				s.mu.Unlock()
				return
			}
			s.frameIdx = (s.frameIdx + 1) % len(s.frames)
			s.renderLine()
			s.mu.Unlock()
		}
	}
}

// Tick manually advances the spinner by one frame and redraws if active.
func (s *Spinner) Tick() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.frameIdx = (s.frameIdx + 1) % len(s.frames)
	if s.isTTY {
		s.renderLine()
	}
}

// SetMessage updates the status message and redraws immediately if active.
func (s *Spinner) SetMessage(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.message = msg
	if s.active && s.isTTY {
		s.renderLine()
	}
}

// UpdateMessage is an alias for SetMessage.
func (s *Spinner) UpdateMessage(msg string) {
	s.SetMessage(msg)
}

// renderLine renders the current frame and message. Must be called with s.mu held.
func (s *Spinner) renderLine() {
	frame := s.frames[s.frameIdx]
	if s.message == "" {
		fmt.Fprintf(s.out, "%s%s", ClearLineCode, frame)
	} else {
		fmt.Fprintf(s.out, "%s%s %s", ClearLineCode, frame, s.message)
	}
}

// Stop stops the spinner animation, clears the line, and restores the terminal cursor.
func (s *Spinner) Stop() {
	s.stopWithAction(func() {
		if s.isTTY {
			fmt.Fprintf(s.out, "%s%s", ClearLineCode, ShowCursorCode)
		}
	})
}

// StopSuccess stops the spinner and prints a success completion line with a green checkmark.
// If msg is empty, the current spinner message is used.
func (s *Spinner) StopSuccess(msg string) {
	s.stopWithAction(func() {
		text := msg
		if text == "" {
			text = s.message
		}
		if s.isTTY {
			fmt.Fprintf(s.out, "%s%s%s %s\n", ClearLineCode, ShowCursorCode, text, Checkmark())
		} else {
			fmt.Fprintf(s.out, "%s %s\n", text, Checkmark())
		}
	})
}

// Success is an alias for StopSuccess.
func (s *Spinner) Success(msg string) {
	s.StopSuccess(msg)
}

// StopFail stops the spinner and prints a failure completion line with a red cross,
// followed by an optional warning hint line.
// If msg is empty, the current spinner message is used.
func (s *Spinner) StopFail(msg, hint string) {
	s.stopWithAction(func() {
		text := msg
		if text == "" {
			text = s.message
		}
		if s.isTTY {
			fmt.Fprintf(s.out, "%s%s%s %s\n", ClearLineCode, ShowCursorCode, text, Cross())
		} else {
			fmt.Fprintf(s.out, "%s %s\n", text, Cross())
		}

		if hint != "" {
			for _, line := range strings.Split(hint, "\n") {
				hintText := fmt.Sprintf("  hint: %s", line)
				fmt.Fprintf(s.out, "%s\n", Warning(hintText))
			}
		}
	})
}

// Fail is an alias for StopFail.
func (s *Spinner) Fail(msg, hint string) {
	s.StopFail(msg, hint)
}

// stopWithAction safely terminates the background loop and executes a final output action.
func (s *Spinner) stopWithAction(action func()) {
	s.mu.Lock()
	if !s.active {
		action()
		s.mu.Unlock()
		return
	}

	s.active = false
	close(s.stopCh)
	s.mu.Unlock()

	// Wait for background loop to finish before final output
	<-s.doneCh

	s.mu.Lock()
	defer s.mu.Unlock()
	action()
}

// RestoreCursor writes the terminal escape code to restore and show the cursor.
func RestoreCursor(w io.Writer) {
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprint(w, ShowCursorCode)
}

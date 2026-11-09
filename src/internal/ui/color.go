package ui

import (
	"fmt"
	"os"
	"sync"
)

// ANSI escape codes
const (
	ResetCode   = "\033[0m"
	WhiteCode   = "\033[37m"
	DimCode     = "\033[2m"
	SuccessCode = "\033[32m" // Green
	DangerCode  = "\033[31m" // Red
	WarningCode = "\033[33m" // Yellow / Orange
)

var (
	colorMu     sync.RWMutex
	colorForced *bool // nil means auto-detect
)

// SetColorEnabled explicitly forces color styling on or off.
// Passing true or false overrides TTY and NO_COLOR checks.
func SetColorEnabled(enabled bool) {
	colorMu.Lock()
	defer colorMu.Unlock()
	b := enabled
	colorForced = &b
}

// ResetColorEnabled resets color styling to auto-detection mode.
func ResetColorEnabled() {
	colorMu.Lock()
	defer colorMu.Unlock()
	colorForced = nil
}

// ColorEnabled reports whether ANSI color escapes should be emitted.
// By default, it auto-detects based on:
// 1. Explicit override via SetColorEnabled
// 2. NO_COLOR environment variable (https://no-color.org)
// 3. TERM="dumb"
// 4. stdout is a terminal (TTY)
func ColorEnabled() bool {
	colorMu.RLock()
	defer colorMu.RUnlock()

	if colorForced != nil {
		return *colorForced
	}

	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	if os.Getenv("TERM") == "dumb" {
		return false
	}

	return IsTerminal(os.Stdout.Fd())
}

// Colorize wraps text with the given ANSI escape code and reset code
// if ColorEnabled() is true. If text is empty or colors are disabled,
// text is returned unmodified.
func Colorize(code, text string) string {
	if text == "" || !ColorEnabled() {
		return text
	}
	return code + text + ResetCode
}

// White returns text formatted in plain white.
func White(s string) string {
	return Colorize(WhiteCode, s)
}

// Whitef formats and returns text in plain white.
func Whitef(format string, a ...any) string {
	return White(fmt.Sprintf(format, a...))
}

// Dim returns text formatted with dim/faint intensity.
func Dim(s string) string {
	return Colorize(DimCode, s)
}

// Dimf formats and returns text with dim/faint intensity.
func Dimf(format string, a ...any) string {
	return Dim(fmt.Sprintf(format, a...))
}

// Success returns text formatted in success (green).
func Success(s string) string {
	return Colorize(SuccessCode, s)
}

// Successf formats and returns text in success (green).
func Successf(format string, a ...any) string {
	return Success(fmt.Sprintf(format, a...))
}

// Danger returns text formatted in danger (red).
func Danger(s string) string {
	return Colorize(DangerCode, s)
}

// Dangerf formats and returns text in danger (red).
func Dangerf(format string, a ...any) string {
	return Danger(fmt.Sprintf(format, a...))
}

// Warning returns text formatted in warning (yellow/orange).
func Warning(s string) string {
	return Colorize(WarningCode, s)
}

// Warningf formats and returns text in warning (yellow/orange).
func Warningf(format string, a ...any) string {
	return Warning(fmt.Sprintf(format, a...))
}

// Checkmark returns a green checkmark glyph ("✓").
func Checkmark() string {
	return Success("✓")
}

// Cross returns a red cross glyph ("✗").
func Cross() string {
	return Danger("✗")
}

// WarningSymbol returns a yellow warning glyph ("⚠").
func WarningSymbol() string {
	return Warning("⚠")
}

// StatusBadge returns a colored status letter following Git conventions.
// M = Warning (yellow), A/U = Success (green), D/C = Danger (red), R = White.
func StatusBadge(letter string) string {
	switch letter {
	case "M":
		return Warning("M")
	case "A":
		return Success("A")
	case "D":
		return Danger("D")
	case "U":
		return Success("U")
	case "R":
		return White("R")
	case "C":
		return Danger("C")
	default:
		return letter
	}
}

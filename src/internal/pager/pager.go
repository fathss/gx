package pager

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

var (
	mu      sync.RWMutex
	noPager bool
)

// SetNoPager sets whether the pager is suppressed (e.g. from --no-pager).
func SetNoPager(disabled bool) {
	mu.Lock()
	defer mu.Unlock()
	noPager = disabled
}

// IsNoPager returns true if the pager is disabled.
func IsNoPager() bool {
	mu.RLock()
	defer mu.RUnlock()
	return noPager
}

// TerminalHeight returns the height (rows) of the terminal, or 24 as a fallback.
func TerminalHeight(fd uintptr) int {
	rows, _ := terminalDimensions(fd)
	if rows > 0 {
		return rows
	}

	if linesStr := os.Getenv("LINES"); linesStr != "" {
		if lines, err := strconv.Atoi(linesStr); err == nil && lines > 0 {
			return lines
		}
	}

	// Fallback to tput
	if out, err := exec.Command("tput", "lines").Output(); err == nil {
		if lines, err := strconv.Atoi(strings.TrimSpace(string(out))); err == nil && lines > 0 {
			return lines
		}
	}

	return 24
}

// ShouldPage reports whether content should be displayed via a pager.
func ShouldPage(content string) bool {
	if IsNoPager() {
		return false
	}
	if !IsTerminal(os.Stdout.Fd()) {
		return false
	}
	term := os.Getenv("TERM")
	if term == "" || term == "dumb" {
		return false
	}
	pagerCmd := getPagerCmd()
	if pagerCmd == "cat" || pagerCmd == "" {
		return false
	}

	trimmed := strings.TrimRight(content, "\n")
	if trimmed == "" {
		return false
	}
	lineCount := strings.Count(trimmed, "\n") + 1
	height := TerminalHeight(os.Stdout.Fd())
	if height > 0 && lineCount < height {
		return false
	}

	return true
}

func getPagerCmd() string {
	if p := os.Getenv("GIT_PAGER"); p != "" {
		return p
	}
	if p := os.Getenv("PAGER"); p != "" {
		return p
	}
	return "less"
}

// Print prints content to stdout, using a pager if appropriate.
func Print(content string) error {
	if !ShouldPage(content) {
		fmt.Println(content)
		return nil
	}

	pagerCmd := getPagerCmd()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", pagerCmd)
	} else {
		cmd = exec.Command("sh", "-c", pagerCmd)
	}

	cmd.Stdin = strings.NewReader(content)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	env := os.Environ()
	if os.Getenv("LESS") == "" {
		env = append(env, "LESS=FRX")
	}
	if os.Getenv("LV") == "" {
		env = append(env, "LV=-c")
	}
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		// If pager failed to launch, fallback to standard output.
		if _, isExitErr := err.(*exec.ExitError); !isExitErr {
			fmt.Println(content)
		}
	}
	return nil
}

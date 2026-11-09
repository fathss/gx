//go:build windows

package pager

import (
	"syscall"
	"unsafe"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	getConsoleScreenBuf = kernel32.NewProc("GetConsoleScreenBufferInfo")
)

type coord struct {
	x int16
	y int16
}

type smallRect struct {
	left   int16
	top    int16
	right  int16
	bottom int16
}

type consoleScreenBufferInfo struct {
	size              coord
	cursorPosition    coord
	attributes        uint16
	window            smallRect
	maximumWindowSize coord
}

// IsTerminal returns true if fd is a terminal.
func IsTerminal(fd uintptr) bool {
	var csbi consoleScreenBufferInfo
	ret, _, _ := getConsoleScreenBuf.Call(fd, uintptr(unsafe.Pointer(&csbi)))
	return ret != 0
}

// terminalDimensions returns the rows and cols of the terminal fd, or (0, 0) on failure.
func terminalDimensions(fd uintptr) (int, int) {
	var csbi consoleScreenBufferInfo
	ret, _, _ := getConsoleScreenBuf.Call(fd, uintptr(unsafe.Pointer(&csbi)))
	if ret != 0 {
		rows := int(csbi.window.bottom - csbi.window.top + 1)
		cols := int(csbi.window.right - csbi.window.left + 1)
		return rows, cols
	}
	return 0, 0
}

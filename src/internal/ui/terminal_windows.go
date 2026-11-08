//go:build windows

package ui

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

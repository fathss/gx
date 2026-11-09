//go:build !windows

package pager

import (
	"syscall"
	"unsafe"
)

// IsTerminal returns true if fd is a terminal.
func IsTerminal(fd uintptr) bool {
	var ws struct {
		Row    uint16
		Col    uint16
		Xpixel uint16
		Ypixel uint16
	}
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&ws)))
	return err == 0
}

// terminalDimensions returns the rows and cols of the terminal fd, or (0, 0) on failure.
func terminalDimensions(fd uintptr) (int, int) {
	var ws struct {
		Row    uint16
		Col    uint16
		Xpixel uint16
		Ypixel uint16
	}
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&ws)))
	if err == 0 {
		return int(ws.Row), int(ws.Col)
	}
	return 0, 0
}

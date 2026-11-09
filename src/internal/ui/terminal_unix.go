//go:build !windows

package ui

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

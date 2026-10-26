//go:build windows

package workflow

// Windows has no portable zero-signal probe; treat the PID as alive and let
// the age fallback in syncLockStale reclaim locks left by crashes.
func processAlive(pid int) bool { return true }

func processAliveReliable() bool { return false }

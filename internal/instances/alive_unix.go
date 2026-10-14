//go:build !windows

package instances

import (
	"errors"
	"syscall"
)

// processAlive reports whether pid runs. Another user's process answers
// EPERM, which still means it runs.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

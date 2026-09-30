//go:build !windows

package training

import (
	"os/exec"
	"syscall"
	"time"
)

// setProcessGroup puts the trainer in its own process group so cancelling
// also stops any workers it started.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// stopProcessGroup asks the group to stop, then kills it after grace.
func stopProcessGroup(cmd *exec.Cmd, grace time.Duration) {
	if cmd.Process == nil {
		return
	}
	pgid := -cmd.Process.Pid
	_ = syscall.Kill(pgid, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		for {
			if err := syscall.Kill(pgid, 0); err != nil {
				close(done)
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()
	select {
	case <-done:
	case <-time.After(grace):
		_ = syscall.Kill(pgid, syscall.SIGKILL)
	}
}

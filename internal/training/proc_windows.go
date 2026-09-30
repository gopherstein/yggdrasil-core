//go:build windows

package training

import (
	"os/exec"
	"time"
)

func setProcessGroup(cmd *exec.Cmd) {}

func stopProcessGroup(cmd *exec.Cmd, grace time.Duration) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

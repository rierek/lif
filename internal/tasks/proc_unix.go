//go:build !windows

package tasks

import (
	"os"
	"os/exec"
	"syscall"
)

// ownGroup puts the step in its own process group and makes a cancel kill the
// whole group, so that no child of the step outlives it.
func ownGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
}

// signalStep sends sig to the step, or to its whole group when group is set.
func signalStep(cmd *exec.Cmd, sig os.Signal, group bool) {
	if s, ok := sig.(syscall.Signal); ok && group {
		_ = syscall.Kill(-cmd.Process.Pid, s)
		return
	}
	_ = cmd.Process.Signal(sig)
}

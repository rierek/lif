//go:build windows

package tasks

import (
	"os"
	"os/exec"
)

// ownGroup does nothing on Windows: a cancel kills the step only.
func ownGroup(*exec.Cmd) {}

// signalStep kills the step: Windows has no signal forwarding.
func signalStep(cmd *exec.Cmd, _ os.Signal, _ bool) { _ = cmd.Process.Kill() }

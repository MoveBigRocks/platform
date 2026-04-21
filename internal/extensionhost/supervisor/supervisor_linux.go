//go:build linux

package supervisor

import (
	"os/exec"
	"syscall"
)

// applyPdeathsig asks the kernel to SIGTERM this child if the supervisor
// process dies unexpectedly. Belt-and-braces with the explicit signal path
// in Supervisor.Stop; covers the case where the core crashes or is
// SIGKILL'd without a chance to run its shutdown hook.
func applyPdeathsig(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Pdeathsig = syscall.SIGTERM
}

//go:build !windows

package localconvert

import (
	"context"
	"os/exec"
	"syscall"
)

func prepareProcess(ctx context.Context, cmd *exec.Cmd) (func() error, func(), error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	kill := func() error {
		if cmd.Process != nil {
			return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return nil
	}
	cmd.Cancel = kill
	return func() error { return ctx.Err() }, func() { _ = kill() }, nil
}

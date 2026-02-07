package netcat

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"runtime"
)

// handleExec wires a network connection to a child process's stdio.
func (nc *NetCat) handleExec(ctx context.Context, conn net.Conn) error {
	var cmd *exec.Cmd

	switch {
	case nc.Config.Command != "":
		if runtime.GOOS == "windows" {
			cmd = exec.CommandContext(ctx, "cmd.exe", "/C", nc.Config.Command)
		} else {
			cmd = exec.CommandContext(ctx, "/bin/sh", "-c", nc.Config.Command)
		}
	case nc.Config.Execute != "":
		cmd = exec.CommandContext(ctx, nc.Config.Execute)
	default:
		return fmt.Errorf("no command specified for exec mode")
	}

	cmd.Stdin = conn
	cmd.Stdout = conn
	cmd.Stderr = conn

	nc.Logger.Debug("exec: %s", cmd.String())

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("exec %q: %w", cmd.Path, err)
	}
	return nil
}

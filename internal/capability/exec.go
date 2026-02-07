package capability

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"

	"gonc/internal/session"
)

// Exec wires a network connection to a child process's stdio.
// Either Program (-e) or Command (-c) must be set.
type Exec struct {
	Program string // -e: execute a program directly
	Command string // -c: execute via the system shell
}

// Handle starts the child process with its stdin/stdout/stderr
// connected to the session's network connection.
func (e *Exec) Handle(ctx context.Context, sess *session.Session) error {
	var cmd *exec.Cmd

	switch {
	case e.Command != "":
		if runtime.GOOS == "windows" {
			cmd = exec.CommandContext(ctx, "cmd.exe", "/C", e.Command)
		} else {
			cmd = exec.CommandContext(ctx, "/bin/sh", "-c", e.Command)
		}
	case e.Program != "":
		cmd = exec.CommandContext(ctx, e.Program)
	default:
		return fmt.Errorf("no command specified for exec mode")
	}

	cmd.Stdin = sess.Conn
	cmd.Stdout = sess.Conn
	cmd.Stderr = sess.Conn

	sess.Logger.Debug("exec: %s", cmd.String())

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("exec %q: %w", cmd.Path, err)
	}
	return nil
}

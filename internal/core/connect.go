package core

import (
	"context"
	"fmt"
	"io"
	"os"

	"gonc/internal/capability"
	"gonc/internal/session"
	"gonc/internal/transport"
	"gonc/util"
)

// ConnectMode dials a remote address and runs a capability on the
// resulting connection — the default client mode.
type ConnectMode struct {
	Dialer     transport.Dialer
	Capability capability.Capability
	Network    string
	Address    string
	Logger     *util.Logger

	// Stdin/Stdout default to os.Stdin/os.Stdout when nil.
	// Override in tests for deterministic I/O.
	Stdin  io.Reader
	Stdout io.Writer
}

func (m *ConnectMode) stdin() io.Reader {
	if m.Stdin != nil {
		return m.Stdin
	}
	return os.Stdin
}

func (m *ConnectMode) stdout() io.Writer {
	if m.Stdout != nil {
		return m.Stdout
	}
	return os.Stdout
}

// Run dials the remote address, creates a session, and hands it to
// the capability.  The transport is closed when Run returns.
func (m *ConnectMode) Run(ctx context.Context) error {
	defer m.Dialer.Close()

	m.Logger.Verbose("connecting to %s (%s)", m.Address, m.Network)

	conn, err := m.Dialer.Dial(ctx, m.Network, m.Address)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", m.Address, err)
	}
	defer conn.Close()

	m.Logger.Verbose("connected to %s", conn.RemoteAddr())

	sess := session.New(conn, m.stdin(), m.stdout(), m.Logger)
	return m.Capability.Handle(ctx, sess)
}

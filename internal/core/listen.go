package core

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"gonc/internal/capability"
	"gonc/internal/session"
	"gonc/util"
)

// ListenMode accepts inbound connections and runs a capability on
// each one.  With KeepOpen=true it spawns a goroutine per connection;
// otherwise it handles one connection and returns.
type ListenMode struct {
	Address    string // ":port"
	Network    string // "tcp" or "udp"
	KeepOpen   bool
	Timeout    time.Duration
	Capability capability.Capability
	Logger     *util.Logger

	// Stdin/Stdout default to os.Stdin/os.Stdout when nil.
	Stdin  io.Reader
	Stdout io.Writer
}

func (m *ListenMode) stdin() io.Reader {
	if m.Stdin != nil {
		return m.Stdin
	}
	return os.Stdin
}

func (m *ListenMode) stdout() io.Writer {
	if m.Stdout != nil {
		return m.Stdout
	}
	return os.Stdout
}

// Run starts listening and dispatches accepted connections to the
// capability.
func (m *ListenMode) Run(ctx context.Context) error {
	if m.Network == "udp" {
		return m.listenUDP(ctx)
	}
	return m.listenTCP(ctx)
}

// ── TCP ──────────────────────────────────────────────────────────────

func (m *ListenMode) listenTCP(ctx context.Context) error {
	ln, err := net.Listen("tcp", m.Address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", m.Address, err)
	}
	defer ln.Close()

	m.Logger.Verbose("listening on %s (tcp)", ln.Addr())

	// Shut the listener down when the context expires.
	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return fmt.Errorf("accept: %w", err)
			}
		}

		m.Logger.Verbose("connection from %s", conn.RemoteAddr())

		if m.KeepOpen {
			go m.serveConn(ctx, conn) //nolint:errcheck
		} else {
			return m.serveConn(ctx, conn)
		}
	}
}

// ── UDP ──────────────────────────────────────────────────────────────

func (m *ListenMode) listenUDP(ctx context.Context) error {
	ua, err := net.ResolveUDPAddr("udp", m.Address)
	if err != nil {
		return fmt.Errorf("resolve UDP: %w", err)
	}
	conn, err := net.ListenUDP("udp", ua)
	if err != nil {
		return fmt.Errorf("listen UDP on %s: %w", m.Address, err)
	}
	defer conn.Close()

	m.Logger.Verbose("listening on %s (udp)", conn.LocalAddr())

	if m.Timeout > 0 {
		conn.SetDeadline(time.Now().Add(m.Timeout)) //nolint:errcheck
	}

	sess := session.New(conn, m.stdin(), m.stdout(), m.Logger)
	return m.Capability.Handle(ctx, sess)
}

// ── Shared ───────────────────────────────────────────────────────────

func (m *ListenMode) serveConn(ctx context.Context, conn net.Conn) error {
	defer conn.Close()

	if m.Timeout > 0 {
		conn.SetDeadline(time.Now().Add(m.Timeout)) //nolint:errcheck
	}

	sess := session.New(conn, m.stdin(), m.stdout(), m.Logger)
	return m.Capability.Handle(ctx, sess)
}

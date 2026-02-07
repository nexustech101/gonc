package netcat

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"gonc/util"
)

// handleServer runs the listen (server) mode.
func (nc *NetCat) handleServer(ctx context.Context) error {
	if nc.Config.UDP {
		return nc.listenUDP(ctx)
	}
	return nc.listenTCP(ctx)
}

// ── TCP ──────────────────────────────────────────────────────────────

func (nc *NetCat) listenTCP(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", nc.Config.LocalPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}
	defer ln.Close()

	nc.Logger.Verbose("listening on %s (tcp)", ln.Addr())

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

		nc.Logger.Verbose("connection from %s", conn.RemoteAddr())

		if nc.Config.KeepOpen {
			go nc.serveConn(ctx, conn) //nolint:errcheck
		} else {
			return nc.serveConn(ctx, conn)
		}
	}
}

// ── UDP ──────────────────────────────────────────────────────────────

func (nc *NetCat) listenUDP(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", nc.Config.LocalPort)
	ua, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return fmt.Errorf("resolve UDP: %w", err)
	}
	conn, err := net.ListenUDP("udp", ua)
	if err != nil {
		return fmt.Errorf("listen UDP on %s: %w", addr, err)
	}
	defer conn.Close()

	nc.Logger.Verbose("listening on %s (udp)", conn.LocalAddr())

	if nc.Config.Timeout > 0 {
		conn.SetDeadline(time.Now().Add(nc.Config.Timeout))
	}

	return util.BidirectionalCopy(ctx, conn, os.Stdin, os.Stdout)
}

// ── Shared ───────────────────────────────────────────────────────────

func (nc *NetCat) serveConn(ctx context.Context, conn net.Conn) error {
	defer conn.Close()

	if nc.Config.Timeout > 0 {
		conn.SetDeadline(time.Now().Add(nc.Config.Timeout))
	}

	if nc.Config.Execute != "" || nc.Config.Command != "" {
		return nc.handleExec(ctx, conn)
	}

	return util.BidirectionalCopy(ctx, conn, os.Stdin, os.Stdout)
}

package netcat

import (
	"context"
	"fmt"
	"net"
	"os"

	"gonc/util"
)

// handleClient runs the connect (client) mode.
func (nc *NetCat) handleClient(ctx context.Context) error {
	if nc.Config.NoDNS && net.ParseIP(nc.Config.Host) == nil {
		return fmt.Errorf("cannot parse %q as an IP address (DNS disabled with -n)", nc.Config.Host)
	}

	addr := util.FormatAddr(nc.Config.Host, nc.Config.Port)
	network := "tcp"
	if nc.Config.UDP {
		network = "udp"
	}

	nc.Logger.Verbose("connecting to %s (%s)", addr, network)

	conn, err := nc.dial(ctx, network, addr)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", addr, err)
	}
	defer conn.Close()

	nc.Logger.Verbose("connected to %s", conn.RemoteAddr())

	// Exec mode: wire the socket to a child process.
	if nc.Config.Execute != "" || nc.Config.Command != "" {
		return nc.handleExec(ctx, conn)
	}

	// Normal interactive / pipe mode.
	return util.BidirectionalCopy(ctx, conn, os.Stdin, os.Stdout)
}

// dial connects to address, optionally through the SSH tunnel.
func (nc *NetCat) dial(ctx context.Context, network, address string) (net.Conn, error) {
	if nc.Tunnel != nil {
		return nc.Tunnel.Dial(ctx, network, address)
	}

	var d net.Dialer
	if nc.Config.Timeout > 0 {
		d.Timeout = nc.Config.Timeout
	}

	// Bind to a specific source port when -p is given in connect mode.
	if nc.Config.LocalPort > 0 && !nc.Config.Listen {
		local := fmt.Sprintf(":%d", nc.Config.LocalPort)
		switch network {
		case "tcp":
			a, err := net.ResolveTCPAddr("tcp", local)
			if err != nil {
				return nil, fmt.Errorf("resolve local addr: %w", err)
			}
			d.LocalAddr = a
		case "udp":
			a, err := net.ResolveUDPAddr("udp", local)
			if err != nil {
				return nil, fmt.Errorf("resolve local addr: %w", err)
			}
			d.LocalAddr = a
		}
	}

	return d.DialContext(ctx, network, address)
}

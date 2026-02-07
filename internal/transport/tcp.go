package transport

import (
	"context"
	"fmt"
	"net"
	"time"
)

// TCPDialer establishes plain TCP connections, optionally binding to a
// specific source port.
type TCPDialer struct {
	Timeout   time.Duration
	LocalPort int // optional source-port binding (0 = ephemeral)
}

// Dial connects to address over TCP.
func (d *TCPDialer) Dial(ctx context.Context, network, address string) (net.Conn, error) {
	dialer := net.Dialer{Timeout: d.Timeout}

	if d.LocalPort > 0 {
		local := fmt.Sprintf(":%d", d.LocalPort)
		a, err := net.ResolveTCPAddr(network, local)
		if err != nil {
			return nil, fmt.Errorf("resolve local addr: %w", err)
		}
		dialer.LocalAddr = a
	}

	return dialer.DialContext(ctx, network, address)
}

// Close is a no-op for stateless TCP dialers.
func (d *TCPDialer) Close() error { return nil }

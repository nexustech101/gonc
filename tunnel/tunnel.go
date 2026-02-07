// Package tunnel defines the Tunnel interface and provides an SSH
// implementation backed by golang.org/x/crypto/ssh.
package tunnel

import (
	"context"
	"net"
)

// Tunnel abstracts an encrypted channel through which TCP connections
// can be forwarded.
type Tunnel interface {
	// Connect establishes the tunnel to the gateway.
	Connect(ctx context.Context) error

	// Dial opens a connection to address through the tunnel.
	Dial(ctx context.Context, network, address string) (net.Conn, error)

	// Close tears down the tunnel and frees resources.
	Close() error

	// IsAlive reports whether the underlying connection is still up.
	IsAlive() bool
}

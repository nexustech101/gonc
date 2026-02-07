// Package transport provides abstractions for network connection
// establishment.  Transports handle the "how" of data movement —
// TCP, UDP, or SSH-tunnelled connections — independent of what
// happens over the connection (which is the capability layer's job).
package transport

import (
	"context"
	"net"
)

// Dialer opens outbound network connections.  Implementations include
// plain TCP/UDP dialers and an SSH-tunnelled dialer that routes
// traffic through an encrypted gateway.
type Dialer interface {
	// Dial establishes a connection to the given network address.
	Dial(ctx context.Context, network, address string) (net.Conn, error)

	// Close releases any long-lived resources held by the dialer
	// (e.g. an SSH session).  Stateless dialers return nil.
	Close() error
}

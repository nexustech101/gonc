package transport

import (
	"context"
	"fmt"
	"net"
	"sync"

	"gonc/tunnel"
	"gonc/util"
)

// SSHDialer routes connections through an SSH tunnel.  The tunnel is
// connected lazily on the first Dial call and torn down on Close.
type SSHDialer struct {
	tunnel    *tunnel.SSHTunnel
	config    *tunnel.SSHConfig
	logger    *util.Logger
	mu        sync.Mutex
	connected bool
}

// NewSSHDialer creates a dialer that forwards connections through an
// SSH tunnel.  The tunnel is not connected until the first Dial.
func NewSSHDialer(cfg *tunnel.SSHConfig, logger *util.Logger) *SSHDialer {
	return &SSHDialer{
		tunnel: tunnel.NewSSHTunnel(cfg, logger),
		config: cfg,
		logger: logger,
	}
}

// connect establishes the SSH tunnel if not already connected.
func (d *SSHDialer) connect(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.connected {
		return nil
	}

	d.logger.Verbose("establishing SSH tunnel to %s@%s:%d",
		d.config.User, d.config.Host, d.config.Port)

	if err := d.tunnel.Connect(ctx); err != nil {
		return fmt.Errorf("tunnel: %w", err)
	}

	d.connected = true
	d.logger.Verbose("SSH tunnel established")
	return nil
}

// Dial connects to address through the SSH tunnel, lazily establishing
// the tunnel on the first call.
func (d *SSHDialer) Dial(ctx context.Context, network, address string) (net.Conn, error) {
	if err := d.connect(ctx); err != nil {
		return nil, err
	}
	return d.tunnel.Dial(ctx, network, address)
}

// Close tears down the underlying SSH tunnel.
func (d *SSHDialer) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.connected {
		d.connected = false
		return d.tunnel.Close()
	}
	return nil
}

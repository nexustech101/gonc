// Package tunnel - reverse_tunnel.go contains the core ReverseTunnel
// type and its lifecycle methods (Start, Wait, Close) plus the accept
// loop.  Supporting logic is split across sibling files:
//
//   - reverse_dial.go      - SSH dialling, gateway validation, message draining
//   - reverse_listener.go  - custom forwarded-tcpip listener
//   - reverse_forwarder.go - connection bridging
//   - reverse_health.go    - keepalive and reconnection
package tunnel

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"gonc/internal/metrics"
	"gonc/util"
)

// ReverseTunnelConfig holds everything needed to establish a reverse
// SSH tunnel that exposes a local service on a remote gateway.
type ReverseTunnelConfig struct {
	// SSH connection parameters.
	SSHConfig *SSHConfig

	// Remote gateway listener.
	RemoteBindAddress string // address to bind on the gateway (default "", i.e. server decides)
	RemotePort        int    // port to bind on the gateway

	// Local service to expose.
	LocalAddress string // local address (default "127.0.0.1")
	LocalPort    int    // local port of the service

	// Behaviour.
	CheckGatewayPorts bool
	KeepAliveInterval time.Duration // 0 disables keepalive
	AutoReconnect     bool
}

// ReverseTunnel forwards connections arriving on a remote SSH gateway
// to a local TCP service.  This is the Go equivalent of ssh -R.
type ReverseTunnel struct {
	config  *ReverseTunnelConfig
	client  *ssh.Client
	listener net.Listener
	logger  *util.Logger
	metrics *metrics.Collector

	ctx    context.Context
	cancel context.CancelFunc

	wg     sync.WaitGroup
	mu     sync.Mutex
	closed bool
}

// NewReverseTunnel creates a reverse tunnel ready to [Start].
// The metrics collector is optional (nil-safe).
func NewReverseTunnel(cfg *ReverseTunnelConfig, logger *util.Logger, m *metrics.Collector) *ReverseTunnel {
	if cfg.LocalAddress == "" {
		cfg.LocalAddress = "127.0.0.1"
	}
	return &ReverseTunnel{config: cfg, logger: logger, metrics: m}
}

// Start connects to the SSH gateway, requests a remote listener, and
// begins forwarding inbound connections to the local service.
func (rt *ReverseTunnel) Start(ctx context.Context) error {
	rt.ctx, rt.cancel = context.WithCancel(ctx)

	// 1. SSH handshake.
	client, err := rt.dialSSH(rt.ctx)
	if err != nil {
		rt.cancel()
		return fmt.Errorf("SSH connection: %w", err)
	}

	rt.mu.Lock()
	rt.client = client
	rt.mu.Unlock()

	// 2. Optional GatewayPorts validation.
	if rt.config.CheckGatewayPorts {
		if err := rt.validateGatewayPorts(); err != nil {
			client.Close()
			rt.cancel()
			return err
		}
	}

	// 3. Request a remote listener via our custom handler that
	//    accepts all forwarded-tcpip channels regardless of the bind
	//    address the server reports (needed for serveo.net et al.).
	listener, err := listenRemoteForward(client, rt.config.RemoteBindAddress, rt.config.RemotePort)
	if err != nil {
		client.Close()
		rt.cancel()
		remoteAddr := fmt.Sprintf("%s:%d", rt.config.RemoteBindAddress, rt.config.RemotePort)
		return fmt.Errorf("remote listen on %s: %w", remoteAddr, err)
	}

	rt.mu.Lock()
	rt.listener = listener
	rt.mu.Unlock()

	remoteAddr := fmt.Sprintf("%s:%d", rt.config.RemoteBindAddress, rt.config.RemotePort)
	localAddr := fmt.Sprintf("%s:%d", rt.config.LocalAddress, rt.config.LocalPort)
	rt.logger.Info("reverse tunnel established: %s (remote) → %s (local)",
		remoteAddr, localAddr)

	// Ensure the listener is closed when the context is cancelled so
	// that a blocking Accept call is unblocked.
	go func() {
		<-rt.ctx.Done()
		rt.mu.Lock()
		if rt.listener != nil {
			rt.listener.Close()
		}
		rt.mu.Unlock()
	}()

	// 4. Keepalive loop (optional).
	if rt.config.KeepAliveInterval > 0 {
		rt.wg.Add(1)
		go rt.keepaliveLoop()
	}

	// 5. Accept loop.
	rt.wg.Add(1)
	go rt.acceptLoop()

	return nil
}

// Wait blocks until every forwarding goroutine has returned.
func (rt *ReverseTunnel) Wait() {
	rt.wg.Wait()
}

// Close tears down the listener, SSH client, and all active forwards.
func (rt *ReverseTunnel) Close() error {
	rt.mu.Lock()
	if rt.closed {
		rt.mu.Unlock()
		return nil
	}
	rt.closed = true
	rt.mu.Unlock()

	if rt.cancel != nil {
		rt.cancel()
	}

	var errs []error

	rt.mu.Lock()
	if rt.listener != nil {
		if err := rt.listener.Close(); err != nil {
			errs = append(errs, fmt.Errorf("listener close: %w", err))
		}
		rt.listener = nil
	}
	rt.mu.Unlock()

	// Give handlers a grace period to finish.
	done := make(chan struct{})
	go func() {
		rt.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		errs = append(errs, fmt.Errorf("timeout waiting for handlers to finish"))
	}

	rt.mu.Lock()
	if rt.client != nil {
		if err := rt.client.Close(); err != nil {
			errs = append(errs, fmt.Errorf("SSH close: %w", err))
		}
		rt.client = nil
	}
	rt.mu.Unlock()

	if len(errs) > 0 {
		return fmt.Errorf("reverse tunnel close: %v", errs)
	}
	return nil
}

// acceptLoop accepts connections from the remote listener and spawns
// a handler goroutine for each one.
func (rt *ReverseTunnel) acceptLoop() {
	defer rt.wg.Done()
	defer rt.cancel() // signal all other goroutines when the loop exits

	for {
		rt.mu.Lock()
		listener := rt.listener
		rt.mu.Unlock()

		if listener == nil {
			return
		}

		remoteConn, err := listener.Accept()
		if err != nil {
			if rt.ctx.Err() != nil {
				return // clean shutdown
			}
			rt.logger.Error("reverse tunnel accept: %v", err)
			rt.metrics.RecordError(fmt.Sprintf("accept: %v", err))

			if rt.config.AutoReconnect {
				if reconnErr := rt.reconnect(); reconnErr != nil {
					rt.logger.Error("reconnect failed, giving up: %v", reconnErr)
					return
				}
				continue // retry with the new listener
			}
			return
		}

		rt.logger.Verbose("reverse tunnel: connection from %s",
			remoteConn.RemoteAddr())
		rt.metrics.ConnectionOpened()

		rt.wg.Add(1)
		go rt.handleConnection(remoteConn)
	}
}

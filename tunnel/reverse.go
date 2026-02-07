// Package tunnel – reverse.go implements remote port forwarding
// (SSH reverse tunnel), equivalent to `ssh -R`.
//
// A reverse tunnel exposes a local TCP service on a remote SSH gateway
// so that external clients connecting to the gateway are transparently
// forwarded back to the local machine.
package tunnel

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

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
	config   *ReverseTunnelConfig
	client   *ssh.Client
	listener net.Listener
	logger   *util.Logger

	ctx    context.Context
	cancel context.CancelFunc

	wg     sync.WaitGroup
	mu     sync.Mutex
	closed bool
}

// NewReverseTunnel creates a reverse tunnel ready to [Start].
func NewReverseTunnel(cfg *ReverseTunnelConfig, logger *util.Logger) *ReverseTunnel {
	if cfg.LocalAddress == "" {
		cfg.LocalAddress = "127.0.0.1"
	}
	return &ReverseTunnel{config: cfg, logger: logger}
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

// ── internal ─────────────────────────────────────────────────────────

// dialSSH establishes an authenticated SSH connection to the gateway.
func (rt *ReverseTunnel) dialSSH(ctx context.Context) (*ssh.Client, error) {
	cfg := rt.config.SSHConfig

	authMethods, err := BuildAuthMethods(cfg)
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}

	hkCb, err := hostKeyCallback(cfg)
	if err != nil {
		return nil, fmt.Errorf("host-key callback: %w", err)
	}

	sshCfg := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            authMethods,
		HostKeyCallback: hkCb,
		Timeout:         cfg.ConnTimeout,
		// Capture the pre-auth banner that services like serveo.net
		// and localhost.run use to display the public URL.
		BannerCallback: func(message string) error {
			rt.logger.Info("%s", message)
			return nil
		},
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	rt.logger.Debug("reverse tunnel: dialing SSH %s as %s", addr, cfg.User)

	var dialer net.Dialer
	tcpConn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("TCP dial %s: %w", addr, err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(tcpConn, addr, sshCfg)
	if err != nil {
		tcpConn.Close()
		return nil, fmt.Errorf("SSH handshake %s: %w", addr, err)
	}

	client := ssh.NewClient(sshConn, chans, reqs)

	// Drain server session messages in the background.  Services
	// like serveo.net send the generated public URL over the SSH
	// session's stdout/stderr after the handshake.
	go rt.drainServerMessages(client)

	return client, nil
}

// validateGatewayPorts performs a best-effort check that the remote
// server allows non-loopback bind addresses (GatewayPorts).
func (rt *ReverseTunnel) validateGatewayPorts() error {
	port, err := util.FindFreePort()
	if err != nil {
		return fmt.Errorf("finding test port: %w", err)
	}

	testAddr := fmt.Sprintf("0.0.0.0:%d", port)
	ln, err := rt.client.Listen("tcp", testAddr)
	if err != nil {
		return fmt.Errorf(
			"GatewayPorts appears disabled on %s – "+
				"set \"GatewayPorts yes\" or \"GatewayPorts clientspecified\" "+
				"in sshd_config: %w",
			rt.config.SSHConfig.Host, err)
	}
	ln.Close()

	rt.logger.Debug("GatewayPorts validation passed")
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

		rt.wg.Add(1)
		go rt.handleConnection(remoteConn)
	}
}

// handleConnection bridges a single remote connection to the local service.
func (rt *ReverseTunnel) handleConnection(remoteConn net.Conn) {
	defer rt.wg.Done()
	defer remoteConn.Close()

	start := time.Now()
	remoteAddr := remoteConn.RemoteAddr().String()

	localTarget := fmt.Sprintf("%s:%d", rt.config.LocalAddress, rt.config.LocalPort)
	localConn, err := net.DialTimeout("tcp", localTarget, 5*time.Second)
	if err != nil {
		rt.logger.Error("reverse tunnel: local dial %s failed: %v",
			localTarget, err)
		return
	}
	defer localConn.Close()

	rt.logger.Info("reverse tunnel: bridging %s ↔ %s", remoteAddr, localTarget)

	bridgeConns(rt.ctx, remoteConn, localConn)

	rt.logger.Info("reverse tunnel: %s closed after %v",
		remoteAddr, time.Since(start))
}

// keepaliveLoop sends periodic SSH keep-alive requests and closes the
// listener if the connection has died, letting acceptLoop handle
// reconnection.
func (rt *ReverseTunnel) keepaliveLoop() {
	defer rt.wg.Done()

	ticker := time.NewTicker(rt.config.KeepAliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rt.ctx.Done():
			return
		case <-ticker.C:
			rt.mu.Lock()
			client := rt.client
			rt.mu.Unlock()

			if client == nil {
				return
			}

			_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
			if err != nil {
				rt.logger.Error("SSH keepalive failed: %v", err)
				// Close the listener to unblock Accept so the
				// acceptLoop can handle reconnection.
				rt.mu.Lock()
				if rt.listener != nil {
					rt.listener.Close()
					rt.listener = nil
				}
				rt.mu.Unlock()
				return
			}
			rt.logger.Debug("SSH keepalive OK")
		}
	}
}

// reconnect tears down the current tunnel and re-establishes it with
// exponential backoff.  It is only called from acceptLoop.
func (rt *ReverseTunnel) reconnect() error {
	rt.logger.Info("reverse tunnel: reconnecting...")

	// Tear down old resources.
	rt.mu.Lock()
	if rt.listener != nil {
		rt.listener.Close()
		rt.listener = nil
	}
	if rt.client != nil {
		rt.client.Close()
		rt.client = nil
	}
	rt.mu.Unlock()

	backoff := time.Second
	const maxBackoff = 60 * time.Second
	const maxAttempts = 10

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if rt.ctx.Err() != nil {
			return rt.ctx.Err()
		}

		client, err := rt.dialSSH(rt.ctx)
		if err != nil {
			rt.logger.Error("reconnect %d/%d SSH: %v", attempt, maxAttempts, err)
			sleepCtx(rt.ctx, backoff)
			backoff = min(backoff*2, maxBackoff)
			continue
		}

		listener, err := listenRemoteForward(client, rt.config.RemoteBindAddress, rt.config.RemotePort)
		if err != nil {
			rt.logger.Error("reconnect %d/%d listen: %v", attempt, maxAttempts, err)
			client.Close()
			sleepCtx(rt.ctx, backoff)
			backoff = min(backoff*2, maxBackoff)
			continue
		}

		rt.mu.Lock()
		rt.client = client
		rt.listener = listener
		rt.mu.Unlock()

		rt.logger.Info("reverse tunnel: reconnected successfully")

		// Restart keepalive with the new client.
		if rt.config.KeepAliveInterval > 0 {
			rt.wg.Add(1)
			go rt.keepaliveLoop()
		}

		return nil
	}

	return fmt.Errorf("failed to reconnect after %d attempts", maxAttempts)
}

// ── custom SSH forward listener ──────────────────────────────────────
//
// Go's ssh.Client.Listen registers forwarded-tcpip channels keyed by
// the exact bind address string it sent.  Many public tunnel services
// (serveo.net, localhost.run) echo back a *different* address (e.g.
// "0.0.0.0" when we sent ""), causing a silent mismatch: the library
// rejects every incoming channel with "no forward for address".
//
// The types below bypass Client.Listen entirely: we register our own
// handler for forwarded-tcpip, send the tcpip-forward global request
// ourselves, and accept all channels unconditionally.

// channelForwardMsg is the wire format for the "tcpip-forward" and
// "cancel-tcpip-forward" global requests (RFC 4254 §7.1).
type channelForwardMsg struct {
	Addr string
	Port uint32
}

// forwardedTCPPayload is the channel-open payload for
// "forwarded-tcpip" (RFC 4254 §7.2).
type forwardedTCPPayload struct {
	Addr       string
	Port       uint32
	OriginAddr string
	OriginPort uint32
}

// sshForwardListener implements net.Listener over SSH forwarded-tcpip
// channels.  It matches all incoming channels regardless of the bind
// address the server reports.
type sshForwardListener struct {
	client   *ssh.Client
	bindAddr string
	bindPort uint32
	incoming <-chan ssh.NewChannel
	done     chan struct{}
	once     sync.Once
}

func (l *sshForwardListener) Accept() (net.Conn, error) {
	select {
	case <-l.done:
		return nil, io.EOF
	case newCh, ok := <-l.incoming:
		if !ok {
			return nil, io.EOF
		}
		ch, reqs, err := newCh.Accept()
		if err != nil {
			return nil, fmt.Errorf("channel accept: %w", err)
		}
		go ssh.DiscardRequests(reqs)

		var raddr net.Addr = &net.TCPAddr{}
		var payload forwardedTCPPayload
		if err := ssh.Unmarshal(newCh.ExtraData(), &payload); err == nil {
			raddr = &net.TCPAddr{
				IP:   net.ParseIP(payload.OriginAddr),
				Port: int(payload.OriginPort),
			}
		}
		return &chanConn{Channel: ch, raddr: raddr}, nil
	}
}

func (l *sshForwardListener) Close() error {
	l.once.Do(func() {
		close(l.done)
		// Best-effort cancel; the connection may already be gone.
		msg := channelForwardMsg{Addr: l.bindAddr, Port: l.bindPort}
		l.client.SendRequest("cancel-tcpip-forward", true, ssh.Marshal(&msg)) //nolint:errcheck
	})
	return nil
}

func (l *sshForwardListener) Addr() net.Addr {
	return &net.TCPAddr{Port: int(l.bindPort)}
}

// chanConn wraps an ssh.Channel to satisfy net.Conn.
type chanConn struct {
	ssh.Channel
	raddr net.Addr
}

func (c *chanConn) LocalAddr() net.Addr                { return &net.TCPAddr{} }
func (c *chanConn) RemoteAddr() net.Addr               { return c.raddr }
func (c *chanConn) SetDeadline(_ time.Time) error      { return nil }
func (c *chanConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *chanConn) SetWriteDeadline(_ time.Time) error { return nil }

// listenRemoteForward sends a tcpip-forward request and returns a
// net.Listener that receives forwarded connections via SSH channels.
// Unlike ssh.Client.Listen it matches all forwarded-tcpip channels
// unconditionally, which is required for public tunnel services.
func listenRemoteForward(client *ssh.Client, bindAddr string, bindPort int) (net.Listener, error) {
	// Register our channel handler BEFORE the library can.
	incoming := client.HandleChannelOpen("forwarded-tcpip")
	if incoming == nil {
		return nil, fmt.Errorf("forwarded-tcpip handler already registered")
	}

	msg := channelForwardMsg{
		Addr: bindAddr,
		Port: uint32(bindPort),
	}
	ok, _, err := client.SendRequest("tcpip-forward", true, ssh.Marshal(&msg))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("tcpip-forward request denied by peer")
	}

	return &sshForwardListener{
		client:   client,
		bindAddr: bindAddr,
		bindPort: uint32(bindPort),
		incoming: incoming,
		done:     make(chan struct{}),
	}, nil
}

// ── helpers ──────────────────────────────────────────────────────────

// bridgeConns copies data bidirectionally between two connections
// until one side closes or the context is cancelled.
func bridgeConns(ctx context.Context, a, b net.Conn) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		io.Copy(b, a) //nolint:errcheck
		cancel()
	}()

	go func() {
		defer wg.Done()
		io.Copy(a, b) //nolint:errcheck
		cancel()
	}()

	<-ctx.Done()
	a.Close()
	b.Close()
	wg.Wait()
}

// sleepCtx sleeps for at most d, returning early if ctx is cancelled.
func sleepCtx(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

// drainServerMessages opens an SSH session and copies its
// stdout/stderr to the logger.  Public tunnel services (serveo.net,
// localhost.run, etc.) use this channel to report the generated URL.
// The goroutine exits silently if the server doesn't support sessions.
func (rt *ReverseTunnel) drainServerMessages(client *ssh.Client) {
	sess, err := client.NewSession()
	if err != nil {
		rt.logger.Debug("reverse tunnel: session for server messages: %v", err)
		return
	}
	defer sess.Close()

	stdout, err := sess.StdoutPipe()
	if err != nil {
		return
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		return
	}

	// Request a shell — some services need it, others allow bare sessions.
	_ = sess.Shell()

	var wg sync.WaitGroup
	printStream := func(r io.Reader) {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			n, readErr := r.Read(buf)
			if n > 0 {
				rt.logger.Info("%s", string(buf[:n]))
			}
			if readErr != nil {
				return
			}
		}
	}

	wg.Add(2)
	go printStream(stdout)
	go printStream(stderr)
	wg.Wait()
}

package tunnel

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	ncerr "gonc/internal/errors"
	"gonc/util"
)

// SSHConfig holds everything needed to dial an SSH gateway.
type SSHConfig struct {
	User          string
	Host          string
	Port          int
	KeyPath       string
	PromptPass    bool
	UseAgent      bool
	StrictHostKey bool
	KnownHosts    string
	ConnTimeout   time.Duration

	// AllowKeyboardInteractive enables adding keyboard-interactive as
	// a fallback auth method.  Public tunnel services (serveo.net,
	// localhost.run) authenticate via keyboard-interactive with empty
	// challenge responses.
	AllowKeyboardInteractive bool
}

// SSHTunnel implements [Tunnel] by opening an SSH connection and
// forwarding traffic with ssh.Client.Dial.
type SSHTunnel struct {
	config *SSHConfig
	client *ssh.Client
	logger *util.Logger
	mu     sync.RWMutex
	alive  bool
}

// NewSSHTunnel creates a tunnel that is ready to [Connect].
func NewSSHTunnel(cfg *SSHConfig, logger *util.Logger) *SSHTunnel {
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	if cfg.ConnTimeout == 0 {
		cfg.ConnTimeout = 30 * time.Second
	}
	return &SSHTunnel{config: cfg, logger: logger}
}

// Connect dials the SSH gateway and completes the handshake.
func (t *SSHTunnel) Connect(ctx context.Context) error {
	authMethods, err := BuildAuthMethods(t.config)
	if err != nil {
		return ncerr.WrapSSH("auth", t.config.Host, t.config.Port, err)
	}

	hkCallback, err := hostKeyCallback(t.config)
	if err != nil {
		return ncerr.WrapSSH("hostkey", t.config.Host, t.config.Port, err)
	}

	sshCfg := &ssh.ClientConfig{
		User:            t.config.User,
		Auth:            authMethods,
		HostKeyCallback: hkCallback,
		Timeout:         t.config.ConnTimeout,
	}

	addr := fmt.Sprintf("%s:%d", t.config.Host, t.config.Port)
	t.logger.Debug("SSH: dialing %s as %s", addr, t.config.User)

	// Use a context-aware TCP dial so callers can cancel.
	var dialer net.Dialer
	tcpConn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return ncerr.Wrap("dial", addr, err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(tcpConn, addr, sshCfg)
	if err != nil {
		tcpConn.Close()
		return ncerr.WrapSSH("handshake", t.config.Host, t.config.Port, err)
	}

	client := ssh.NewClient(sshConn, chans, reqs)

	t.mu.Lock()
	t.client = client
	t.alive = true
	t.mu.Unlock()

	go t.monitor()

	return nil
}

// Dial forwards a connection through the tunnel.
func (t *SSHTunnel) Dial(ctx context.Context, network, address string) (net.Conn, error) {
	t.mu.RLock()
	client := t.client
	alive := t.alive
	t.mu.RUnlock()

	if !alive || client == nil {
		return nil, ncerr.ErrNotConnected
	}

	t.logger.Debug("tunnel: dialing %s %s", network, address)
	conn, err := client.Dial(network, address)
	if err != nil {
		return nil, fmt.Errorf("tunnel dial %s: %w", address, err)
	}
	return conn, nil
}

// Close shuts down the SSH connection.
func (t *SSHTunnel) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.alive = false
	if t.client != nil {
		err := t.client.Close()
		t.client = nil
		return err
	}
	return nil
}

// IsAlive reports whether the tunnel is still connected.
func (t *SSHTunnel) IsAlive() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.alive
}

// monitor blocks until the SSH connection closes and flips the alive flag.
func (t *SSHTunnel) monitor() {
	t.mu.RLock()
	client := t.client
	t.mu.RUnlock()
	if client == nil {
		return
	}

	err := client.Wait()

	t.mu.Lock()
	t.alive = false
	t.mu.Unlock()

	if err != nil {
		t.logger.Debug("SSH tunnel closed: %v", err)
	} else {
		t.logger.Debug("SSH tunnel closed")
	}
}

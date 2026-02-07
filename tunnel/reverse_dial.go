package tunnel

// reverse_dial.go - SSH dialling, gateway validation, and server
// message draining for the reverse tunnel.

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"

	"golang.org/x/crypto/ssh"

	"gonc/util"
)

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

	// Drain server session messages in the background.
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
			"GatewayPorts appears disabled on %s - "+
				"set \"GatewayPorts yes\" or \"GatewayPorts clientspecified\" "+
				"in sshd_config: %w",
			rt.config.SSHConfig.Host, err)
	}
	ln.Close()

	rt.logger.Debug("GatewayPorts validation passed")
	return nil
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

package tunnel

// reverse_listener.go - custom SSH forwarded-tcpip listener.
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

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// ── Wire format structs (RFC 4254) ──────────────────────────────────

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

// ── sshForwardListener ──────────────────────────────────────────────

// sshForwardListener implements [net.Listener] over SSH forwarded-tcpip
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

// Accept waits for the next forwarded connection from the remote.
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

// Close cancels the remote port forward and unblocks Accept.
func (l *sshForwardListener) Close() error {
	l.once.Do(func() {
		close(l.done)
		// Best-effort cancel; the connection may already be gone.
		msg := channelForwardMsg{Addr: l.bindAddr, Port: l.bindPort}
		l.client.SendRequest("cancel-tcpip-forward", true, ssh.Marshal(&msg)) //nolint:errcheck
	})
	return nil
}

// Addr returns the listener's network address.
func (l *sshForwardListener) Addr() net.Addr {
	return &net.TCPAddr{Port: int(l.bindPort)}
}

// ── chanConn ─────────────────────────────────────────────────────────

// chanConn wraps an [ssh.Channel] to satisfy [net.Conn].
type chanConn struct {
	ssh.Channel
	raddr net.Addr
}

func (c *chanConn) LocalAddr() net.Addr                { return &net.TCPAddr{} }
func (c *chanConn) RemoteAddr() net.Addr               { return c.raddr }
func (c *chanConn) SetDeadline(_ time.Time) error      { return nil }
func (c *chanConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *chanConn) SetWriteDeadline(_ time.Time) error { return nil }

// ── Constructor ──────────────────────────────────────────────────────

// listenRemoteForward sends a tcpip-forward request and returns a
// [net.Listener] that receives forwarded connections via SSH channels.
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

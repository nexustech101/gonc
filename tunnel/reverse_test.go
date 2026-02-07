package tunnel

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"gonc/util"
)

// ── bridgeConns ──────────────────────────────────────────────────────

func TestBridgeConnsForward(t *testing.T) {
	aServer, aClient := net.Pipe()
	bServer, bClient := net.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		bridgeConns(ctx, aServer, bServer)
		close(done)
	}()

	// Data written to aClient should appear on bClient.
	msg := []byte("hello reverse tunnel")
	go func() {
		aClient.Write(msg) //nolint:errcheck
		aClient.Close()
	}()

	got, err := io.ReadAll(bClient)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != string(msg) {
		t.Errorf("forward: got %q, want %q", got, msg)
	}

	bClient.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("bridgeConns did not return")
	}
}

func TestBridgeConnsBidirectional(t *testing.T) {
	aServer, aClient := net.Pipe()
	bServer, bClient := net.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go bridgeConns(ctx, aServer, bServer)

	// a → b
	msgAB := []byte("from-A")
	go aClient.Write(msgAB) //nolint:errcheck

	buf := make([]byte, len(msgAB))
	if _, err := io.ReadFull(bClient, buf); err != nil {
		t.Fatalf("A→B read: %v", err)
	}
	if string(buf) != string(msgAB) {
		t.Errorf("A→B: got %q, want %q", buf, msgAB)
	}

	// b → a
	msgBA := []byte("from-B")
	go bClient.Write(msgBA) //nolint:errcheck

	buf = make([]byte, len(msgBA))
	if _, err := io.ReadFull(aClient, buf); err != nil {
		t.Fatalf("B→A read: %v", err)
	}
	if string(buf) != string(msgBA) {
		t.Errorf("B→A: got %q, want %q", buf, msgBA)
	}

	aClient.Close()
	bClient.Close()
}

func TestBridgeConnsContextCancel(t *testing.T) {
	aServer, aClient := net.Pipe()
	bServer, bClient := net.Pipe()
	defer aClient.Close()
	defer bClient.Close()

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		bridgeConns(ctx, aServer, bServer)
		close(done)
	}()

	// Cancel the context; bridge should tear down promptly.
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("bridgeConns did not return after context cancel")
	}
}

// ── handleConnection ─────────────────────────────────────────────────

func TestHandleConnectionForwardsToLocal(t *testing.T) {
	// Start a local echo server.
	echoLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echoLn.Close()

	go func() {
		for {
			conn, err := echoLn.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(c, c) //nolint:errcheck
			}(conn)
		}
	}()

	localPort := echoLn.Addr().(*net.TCPAddr).Port

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rt := &ReverseTunnel{
		config: &ReverseTunnelConfig{
			LocalAddress: "127.0.0.1",
			LocalPort:    localPort,
		},
		logger: util.NewLogger(0),
		ctx:    ctx,
		cancel: cancel,
	}

	// Simulate a remote connection with a pipe.
	remoteServer, remoteClient := net.Pipe()

	rt.wg.Add(1)
	go rt.handleConnection(remoteServer)

	// Send data and expect it echoed back.
	payload := []byte("echo-test-data")
	if _, err := remoteClient.Write(payload); err != nil {
		t.Fatalf("write: %v", err)
	}

	buf := make([]byte, len(payload))
	if _, err := io.ReadFull(remoteClient, buf); err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(buf) != string(payload) {
		t.Errorf("echo: got %q, want %q", buf, payload)
	}

	remoteClient.Close()
	rt.wg.Wait()
}

func TestHandleConnectionLocalRefused(t *testing.T) {
	// Use a port where nothing is listening.
	freePort, err := util.FindFreePort()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rt := &ReverseTunnel{
		config: &ReverseTunnelConfig{
			LocalAddress: "127.0.0.1",
			LocalPort:    freePort,
		},
		logger: util.NewLogger(0),
		ctx:    ctx,
		cancel: cancel,
	}

	remoteServer, remoteClient := net.Pipe()
	defer remoteClient.Close()

	rt.wg.Add(1)
	done := make(chan struct{})
	go func() {
		rt.handleConnection(remoteServer)
		close(done)
	}()

	// handleConnection should return quickly because the local dial fails.
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handleConnection did not return after local dial failure")
	}

	rt.wg.Wait()
}

// ── sleepCtx ─────────────────────────────────────────────────────────

func TestSleepCtxFull(t *testing.T) {
	ctx := context.Background()
	start := time.Now()
	sleepCtx(ctx, 50*time.Millisecond)
	if elapsed := time.Since(start); elapsed < 40*time.Millisecond {
		t.Errorf("sleepCtx returned too early: %v", elapsed)
	}
}

func TestSleepCtxCancelledEarly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	start := time.Now()

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	sleepCtx(ctx, 10*time.Second)

	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("sleepCtx took %v, expected early return on cancel", elapsed)
	}
}

// ── NewReverseTunnel ─────────────────────────────────────────────────

func TestNewReverseTunnelDefaults(t *testing.T) {
	rt := NewReverseTunnel(&ReverseTunnelConfig{
		SSHConfig:  &SSHConfig{Host: "gw", Port: 22, User: "u"},
		RemotePort: 9000,
		LocalPort:  8080,
	}, util.NewLogger(0), nil)

	if rt.config.RemoteBindAddress != "" {
		t.Errorf("RemoteBindAddress = %q, want %q",
			rt.config.RemoteBindAddress, "")
	}
	if rt.config.LocalAddress != "127.0.0.1" {
		t.Errorf("LocalAddress = %q, want %q",
			rt.config.LocalAddress, "127.0.0.1")
	}
}

func TestNewReverseTunnelPreservesExplicit(t *testing.T) {
	rt := NewReverseTunnel(&ReverseTunnelConfig{
		SSHConfig:         &SSHConfig{Host: "gw", Port: 22, User: "u"},
		RemoteBindAddress: "10.0.1.5",
		RemotePort:        3306,
		LocalAddress:      "192.168.1.100",
		LocalPort:         3306,
	}, util.NewLogger(0), nil)

	if rt.config.RemoteBindAddress != "10.0.1.5" {
		t.Errorf("RemoteBindAddress = %q, want %q",
			rt.config.RemoteBindAddress, "10.0.1.5")
	}
	if rt.config.LocalAddress != "192.168.1.100" {
		t.Errorf("LocalAddress = %q, want %q",
			rt.config.LocalAddress, "192.168.1.100")
	}
}

// ── Close idempotency ────────────────────────────────────────────────

func TestReverseTunnelCloseIdempotent(t *testing.T) {
	rt := NewReverseTunnel(&ReverseTunnelConfig{
		SSHConfig:  &SSHConfig{Host: "gw", Port: 22, User: "u"},
		RemotePort: 9000,
		LocalPort:  8080,
	}, util.NewLogger(0), nil)

	// Close without Start should be safe.
	if err := rt.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := rt.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

package transport

import (
	"context"
	"io"
	"net"
	"testing"
	"time"
)

// TestTCPDialer_Connect verifies that TCPDialer can reach a local
// TCP server and exchange data.
func TestTCPDialer_Connect(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	// Server: accept, send greeting, close.
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		conn.Write([]byte("hello from server\n")) //nolint:errcheck
	}()

	d := &TCPDialer{Timeout: 2 * time.Second}
	ctx := context.Background()

	conn, err := d.Dial(ctx, "tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("read: %v", err)
	}
	if got := string(buf[:n]); got != "hello from server\n" {
		t.Errorf("got %q, want %q", got, "hello from server\n")
	}
}

// TestTCPDialer_ContextCancel verifies that a cancelled context stops the dial.
func TestTCPDialer_ContextCancel(t *testing.T) {
	d := &TCPDialer{Timeout: 5 * time.Second}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := d.Dial(ctx, "tcp", "127.0.0.1:1")
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

// TestTCPDialer_Close verifies Close is a no-op and returns nil.
func TestTCPDialer_Close(t *testing.T) {
	d := &TCPDialer{}
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestUDPDialer_Connect verifies that UDPDialer returns a connection.
func TestUDPDialer_Connect(t *testing.T) {
	ua, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.ListenUDP("udp", ua)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	d := &UDPDialer{Timeout: 2 * time.Second}
	conn, err := d.Dial(context.Background(), "udp", ln.LocalAddr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Write data through the UDP connection.
	_, err = conn.Write([]byte("ping"))
	if err != nil {
		t.Fatalf("write: %v", err)
	}
}

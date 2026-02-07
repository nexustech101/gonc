package netcat

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"gonc/config"
	"gonc/util"
)

// TestClientConnect verifies that the client can reach a local TCP server.
func TestClientConnect(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port

	// Server: accept one conn, send a greeting, close.
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		conn.Write([]byte("hello from server\n"))
	}()

	// Build a NetCat in connect mode that reads from an empty stdin.
	cfg := &config.Config{
		Host:    "127.0.0.1",
		Port:    port,
		Timeout: 2 * time.Second,
		NoDNS:   true,
	}
	logger := util.NewLogger(0)
	nc := New(cfg, nil, logger)

	// Override stdin/stdout for the test: we do this by calling
	// the unexported dial + BidirectionalCopy manually.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	conn, err := nc.dial(ctx, "tcp", "127.0.0.1:"+itoa(port))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("read: %v", err)
	}
	got := string(buf[:n])
	if got != "hello from server\n" {
		t.Errorf("got %q, want %q", got, "hello from server\n")
	}
}

// TestClientSendData verifies data flows from client to server.
func TestClientSendData(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	received := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		var buf bytes.Buffer
		io.Copy(&buf, conn)
		received <- buf.String()
	}()

	cfg := &config.Config{
		Host:    "127.0.0.1",
		Port:    port,
		NoDNS:   true,
		Timeout: 2 * time.Second,
	}
	nc := New(cfg, nil, util.NewLogger(0))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	conn, err := nc.dial(ctx, "tcp", "127.0.0.1:"+itoa(port))
	if err != nil {
		t.Fatal(err)
	}
	conn.Write([]byte("payload from client"))
	conn.Close()

	select {
	case got := <-received:
		if got != "payload from client" {
			t.Errorf("server got %q", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for data")
	}
}

// itoa avoids importing strconv for a tiny helper.
func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

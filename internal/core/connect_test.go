package core

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"gonc/internal/capability"
	"gonc/internal/transport"
	"gonc/util"
)

// TestConnectMode_TCP verifies end-to-end connect mode with Relay.
func TestConnectMode_TCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	// Server: accept one conn, send greeting, close.
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		conn.Write([]byte("hello from server\n")) //nolint:errcheck
	}()

	port := ln.Addr().(*net.TCPAddr).Port
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	input := bytes.NewBufferString("")
	output := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	mode := &ConnectMode{
		Dialer:     &transport.TCPDialer{Timeout: 2 * time.Second},
		Capability: &capability.Relay{},
		Network:    "tcp",
		Address:    addr,
		Logger:     util.NewLogger(0),
		Stdin:      input,
		Stdout:     output,
	}

	err = mode.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := output.String(); got != "hello from server\n" {
		t.Errorf("output = %q, want %q", got, "hello from server\n")
	}
}

// TestConnectMode_SendData verifies data flows from client to server.
func TestConnectMode_SendData(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	addr := fmt.Sprintf("127.0.0.1:%d", port)

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

	input := bytes.NewBufferString("payload from client")
	output := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	mode := &ConnectMode{
		Dialer:     &transport.TCPDialer{Timeout: 2 * time.Second},
		Capability: &capability.Relay{},
		Network:    "tcp",
		Address:    addr,
		Logger:     util.NewLogger(0),
		Stdin:      input,
		Stdout:     output,
	}

	_ = mode.Run(ctx)

	select {
	case got := <-received:
		if got != "payload from client" {
			t.Errorf("server got %q", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for data")
	}
}

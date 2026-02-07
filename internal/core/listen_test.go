package core

import (
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"gonc/internal/capability"
	"gonc/util"
)

// TestListenMode_TCP verifies that ListenMode accepts a connection.
func TestListenMode_TCP(t *testing.T) {
	port, err := util.FindFreePort()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	mode := &ListenMode{
		Address:    fmt.Sprintf(":%d", port),
		Network:    "tcp",
		Timeout:    2 * time.Second,
		Capability: &capability.Relay{},
		Logger:     util.NewLogger(0),
	}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- mode.Run(ctx)
	}()

	// Give the server a moment to start listening.
	time.Sleep(100 * time.Millisecond)

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	conn.Write([]byte("test message")) //nolint:errcheck
	conn.Close()

	cancel()

	select {
	case err := <-serverErr:
		if err != nil {
			t.Logf("server returned (expected after cancel): %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not shut down in time")
	}
}

// TestListenMode_KeepOpen verifies -k accepts multiple connections.
func TestListenMode_KeepOpen(t *testing.T) {
	port, err := util.FindFreePort()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	mode := &ListenMode{
		Address:    fmt.Sprintf(":%d", port),
		Network:    "tcp",
		KeepOpen:   true,
		Timeout:    1 * time.Second,
		Capability: &capability.Relay{},
		Logger:     util.NewLogger(0),
	}

	go mode.Run(ctx) //nolint:errcheck

	time.Sleep(100 * time.Millisecond)

	for i := 0; i < 3; i++ {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 1*time.Second)
		if err != nil {
			t.Fatalf("conn %d dial: %v", i, err)
		}
		conn.Write([]byte("ping")) //nolint:errcheck
		io.ReadAll(conn)           // drain
		conn.Close()
	}
}

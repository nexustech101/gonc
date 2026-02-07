package netcat

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"gonc/config"
	"gonc/util"
)

// TestServerAccept verifies that the server accepts a connection and
// can exchange data.
func TestServerAccept(t *testing.T) {
	port, err := util.FindFreePort()
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Listen:    true,
		LocalPort: port,
		Timeout:   2 * time.Second,
	}
	logger := util.NewLogger(0)
	nc := New(cfg, nil, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	// Start the server in a goroutine (it blocks on Accept).
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- nc.listenTCP(ctx)
	}()

	// Give the server a moment to start listening.
	time.Sleep(100 * time.Millisecond)

	// Connect as a client.
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+itoa(port), 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	// Write something and close.
	conn.Write([]byte("test message"))
	conn.Close()

	// The server's serveConn should finish once the client closes.
	cancel() // also signal context done

	select {
	case err := <-serverErr:
		if err != nil {
			t.Logf("server returned (expected after cancel): %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not shut down in time")
	}
}

// TestServerKeepOpen verifies -k accepts multiple connections.
func TestServerKeepOpen(t *testing.T) {
	port, err := util.FindFreePort()
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Listen:    true,
		LocalPort: port,
		KeepOpen:  true,
		Timeout:   1 * time.Second,
	}
	nc := New(cfg, nil, util.NewLogger(0))

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	go nc.listenTCP(ctx) //nolint:errcheck

	time.Sleep(100 * time.Millisecond)

	for i := 0; i < 3; i++ {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:"+itoa(port), 1*time.Second)
		if err != nil {
			t.Fatalf("conn %d dial: %v", i, err)
		}
		conn.Write([]byte("ping"))
		io.ReadAll(conn) // drain
		conn.Close()
	}
}

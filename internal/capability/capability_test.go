package capability

import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"
	"time"

	"gonc/internal/session"
	"gonc/util"
)

// TestRelay_BidirectionalCopy verifies Relay shuttles data via the
// session's I/O endpoints.
func TestRelay_BidirectionalCopy(t *testing.T) {
	// Set up a local TCP echo server.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		io.Copy(conn, conn) // echo
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	input := bytes.NewBufferString("hello relay\n")
	output := &bytes.Buffer{}
	logger := util.NewLogger(0)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	sess := session.New(conn, input, output, logger)
	relay := &Relay{}

	err = relay.Handle(ctx, sess)
	if err != nil {
		t.Fatalf("Relay.Handle: %v", err)
	}

	if got := output.String(); got != "hello relay\n" {
		t.Errorf("output = %q, want %q", got, "hello relay\n")
	}
}

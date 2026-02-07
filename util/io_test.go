package util

import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"
	"time"
)

func TestBidirectionalCopy(t *testing.T) {
	// Set up a TCP server that echoes data.
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

	// Connect as client.
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	input := bytes.NewBufferString("hello world\n")
	output := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// BidirectionalCopy: input → conn → echo → output
	// When input is exhausted the write side half-closes; the echo
	// server then sees EOF and closes its side, ending the copy.
	err = BidirectionalCopy(ctx, conn, input, output)
	if err != nil {
		t.Fatalf("BidirectionalCopy: %v", err)
	}

	if got := output.String(); got != "hello world\n" {
		t.Errorf("output = %q, want %q", got, "hello world\n")
	}
}

func TestIsHarmless(t *testing.T) {
	if !isHarmless(nil) {
		t.Error("nil should be harmless")
	}
	if !isHarmless(io.EOF) {
		t.Error("io.EOF should be harmless")
	}
	if !isHarmless(net.ErrClosed) {
		t.Error("net.ErrClosed should be harmless")
	}
	if isHarmless(io.ErrUnexpectedEOF) {
		t.Error("ErrUnexpectedEOF should NOT be harmless")
	}
}

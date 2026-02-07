package tunnel

import (
	"context"
	"io"
	"net"
	"testing"
)

// BenchmarkBridgeConns measures bidirectional copy throughput between
// two TCP connections through an echo server.
func BenchmarkBridgeConns(b *testing.B) {
	// TCP echo server.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	defer ln.Close()

	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(c, c) //nolint:errcheck
			}(c)
		}
	}()

	payload := make([]byte, 32*1024) // 32 KiB
	for i := range payload {
		payload[i] = byte(i)
	}

	b.SetBytes(int64(len(payload)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		left, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			b.Fatal(err)
		}
		right, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			left.Close()
			b.Fatal(err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		// Write payload into left, let bridgeConns forward to right.
		go func() {
			left.Write(payload) //nolint:errcheck
			left.(*net.TCPConn).CloseWrite()
		}()

		aToB, bToA := bridgeConns(ctx, left, right)
		cancel()
		_ = aToB
		_ = bToA
	}
}

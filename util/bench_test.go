package util

import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"
)

// BenchmarkBidirectionalCopy measures throughput of the bidirectional
// copy loop that is the hot path for all data forwarding.
func BenchmarkBidirectionalCopy(b *testing.B) {
	// Create a TCP echo server.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(c, c) //nolint:errcheck
			}(conn)
		}
	}()

	payload := bytes.Repeat([]byte("X"), DefaultBufSize)

	b.SetBytes(int64(len(payload)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		conn, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			b.Fatal(err)
		}

		input := bytes.NewReader(payload)
		output := io.Discard

		ctx, cancel := context.WithCancel(context.Background())
		BidirectionalCopy(ctx, conn, input, output) //nolint:errcheck
		cancel()
	}
}

// BenchmarkBufPool measures the allocation advantage of sync.Pool
// buffer reuse versus fresh allocation.
func BenchmarkBufPool(b *testing.B) {
	b.Run("pool", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			buf := GetBuf()
			_ = (*buf)[0]
			PutBuf(buf)
		}
	})
	b.Run("alloc", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			buf := make([]byte, DefaultBufSize)
			_ = buf[0]
		}
	})
}

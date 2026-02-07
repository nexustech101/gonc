package netcat

import (
	"context"
	"net"
	"testing"
	"time"
)

// BenchmarkScanPorts measures the scanning throughput.
func BenchmarkScanPorts(b *testing.B) {
	// Create a listener so at least one port is "open".
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	defer ln.Close()

	openPort := ln.Addr().(*net.TCPAddr).Port
	ports := []int{openPort, 1} // one open, one closed

	dialFn := func(ctx context.Context, network, address string) (net.Conn, error) {
		d := net.Dialer{Timeout: 200 * time.Millisecond}
		return d.DialContext(ctx, network, address)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ScanPorts(context.Background(), "127.0.0.1", ports, 500*time.Millisecond, dialFn)
	}
}

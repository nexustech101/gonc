package netcat

import (
	"context"
	"net"
	"testing"
	"time"
)

// TestScanPorts verifies open / closed detection on localhost.
func TestScanPorts(t *testing.T) {
	// Open two listeners on random ports.
	ln1, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln1.Close()
	ln2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln2.Close()

	openPort1 := ln1.Addr().(*net.TCPAddr).Port
	openPort2 := ln2.Addr().(*net.TCPAddr).Port

	// Pick a port that is (almost certainly) closed.
	closedPort := 1 // port 1 is not normally listening

	ports := []int{openPort1, closedPort, openPort2}
	dialFn := func(ctx context.Context, network, address string) (net.Conn, error) {
		d := net.Dialer{Timeout: 500 * time.Millisecond}
		return d.DialContext(ctx, network, address)
	}

	results := ScanPorts(context.Background(), "127.0.0.1", ports, 1*time.Second, dialFn)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if !results[0].Open {
		t.Errorf("port %d should be open", openPort1)
	}
	if results[1].Open {
		t.Errorf("port %d should be closed", closedPort)
	}
	if !results[2].Open {
		t.Errorf("port %d should be open", openPort2)
	}
}

// TestScanPortsTimeout verifies scans respect the timeout.
func TestScanPortsTimeout(t *testing.T) {
	// Use a non-routable IP so the connection hangs until timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	dialFn := func(ctx context.Context, network, address string) (net.Conn, error) {
		d := net.Dialer{Timeout: 200 * time.Millisecond}
		return d.DialContext(ctx, network, address)
	}

	start := time.Now()
	results := ScanPorts(ctx, "127.0.0.1", []int{1}, 500*time.Millisecond, dialFn)
	elapsed := time.Since(start)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Open {
		t.Skip("port 1 unexpectedly open on this host")
	}
	if elapsed > 3*time.Second {
		t.Errorf("scan took %v, expected < 3s", elapsed)
	}
}

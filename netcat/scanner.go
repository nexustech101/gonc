package netcat

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"gonc/config"
	"gonc/util"
)

const (
	defaultScanTimeout = config.DefaultScanTimeout
	maxConcurrentScans = config.DefaultMaxConcurrentScans
)

// DialFunc establishes a network connection.
type DialFunc func(ctx context.Context, network, address string) (net.Conn, error)

// ScanResult records whether a single port is open.
type ScanResult struct {
	Port int
	Open bool
	Err  error
}

// handleScan runs the port-scanning (-z) mode.
func (nc *NetCat) handleScan(ctx context.Context) error {
	if nc.Config.NoDNS && net.ParseIP(nc.Config.Host) == nil {
		return fmt.Errorf("cannot parse %q as an IP address (DNS disabled with -n)", nc.Config.Host)
	}

	ports := nc.Config.AllPorts()
	if len(ports) == 0 && nc.Config.Port > 0 {
		ports = []int{nc.Config.Port}
	}
	if len(ports) == 0 {
		return fmt.Errorf("no ports specified for scanning")
	}

	timeout := nc.Config.Timeout
	if timeout == 0 {
		timeout = defaultScanTimeout
	}

	nc.Logger.Verbose("scanning %s - %d port(s)", nc.Config.Host, len(ports))

	// Build the dial function (through tunnel or direct).
	var dialFn DialFunc
	if nc.Tunnel != nil {
		dialFn = nc.Tunnel.Dial
	} else {
		dialFn = func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: timeout}
			return d.DialContext(ctx, network, address)
		}
	}

	results := ScanPorts(ctx, nc.Config.Host, ports, timeout, dialFn)

	open := 0
	for _, r := range results {
		if r.Open {
			open++
			nc.Logger.Info("%s %d/tcp open", nc.Config.Host, r.Port)
		} else if nc.Config.Verbose >= 2 {
			nc.Logger.Verbose("%s %d/tcp closed - %v", nc.Config.Host, r.Port, r.Err)
		}
	}

	if open == 0 && nc.Config.Verbose >= 1 {
		nc.Logger.Info("no open ports found on %s", nc.Config.Host)
	}
	return nil
}

// ScanPorts probes every port concurrently and returns results in the
// same order as the input slice.
func ScanPorts(ctx context.Context, host string, ports []int, timeout time.Duration, dial DialFunc) []ScanResult {
	results := make([]ScanResult, len(ports))
	sem := make(chan struct{}, maxConcurrentScans)
	var wg sync.WaitGroup

	for i, port := range ports {
		wg.Add(1)
		go func(idx, p int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			addr := util.FormatAddr(host, p)
			scanCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			conn, err := dial(scanCtx, "tcp", addr)
			if err != nil {
				results[idx] = ScanResult{Port: p, Open: false, Err: err}
				return
			}
			conn.Close()
			results[idx] = ScanResult{Port: p, Open: true}
		}(i, port)
	}

	wg.Wait()
	return results
}

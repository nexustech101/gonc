package core

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"gonc/config"
	"gonc/internal/transport"
	"gonc/util"
)

// DialFunc establishes a network connection.
type DialFunc func(ctx context.Context, network, address string) (net.Conn, error)

// ScanResult records whether a single port is open.
type ScanResult struct {
	Port int
	Open bool
	Err  error
}

// ScanMode probes a set of TCP ports on a target host and reports
// which are open.
type ScanMode struct {
	Dialer  transport.Dialer
	Host    string
	Ports   []int
	Timeout time.Duration
	Logger  *util.Logger
	Verbose int
}

// Run scans all configured ports and logs the results.  The
// underlying transport is closed when Run returns.
func (m *ScanMode) Run(ctx context.Context) error {
	defer m.Dialer.Close()

	if len(m.Ports) == 0 {
		return fmt.Errorf("no ports specified for scanning")
	}

	timeout := m.Timeout
	if timeout == 0 {
		timeout = config.DefaultScanTimeout
	}

	m.Logger.Verbose("scanning %s - %d port(s)", m.Host, len(m.Ports))

	results := ScanPorts(ctx, m.Host, m.Ports, timeout, m.Dialer.Dial)

	open := 0
	for _, r := range results {
		if r.Open {
			open++
			m.Logger.Info("%s %d/tcp open", m.Host, r.Port)
		} else if m.Verbose >= 2 {
			m.Logger.Verbose("%s %d/tcp closed - %v", m.Host, r.Port, r.Err)
		}
	}

	if open == 0 && m.Verbose >= 1 {
		m.Logger.Info("no open ports found on %s", m.Host)
	}
	return nil
}

// ScanPorts probes every port concurrently and returns results in the
// same order as the input slice.
func ScanPorts(ctx context.Context, host string, ports []int, timeout time.Duration, dial DialFunc) []ScanResult {
	results := make([]ScanResult, len(ports))
	sem := make(chan struct{}, config.DefaultMaxConcurrentScans)
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

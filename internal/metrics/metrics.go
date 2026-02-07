// Package metrics provides lightweight, lock-free counters and gauges
// for tracking runtime statistics of a gonc session.
//
// All methods are safe for concurrent use.  A nil *Collector is a
// valid no-op receiver, so callers never need to nil-check.
package metrics

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"
)

// Collector tracks runtime metrics for a gonc session.
// A nil Collector is safe to use — all methods become no-ops.
type Collector struct {
	connectionsActive atomic.Int64
	connectionsTotal  atomic.Int64
	bytesIn           atomic.Int64
	bytesOut          atomic.Int64
	tunnelReconnects  atomic.Int64
	errorsTotal       atomic.Int64

	mu              sync.RWMutex
	startTime       time.Time
	lastHealthCheck time.Time
	lastError       time.Time
	lastErrorMsg    string
}

// New creates a metrics collector with the start time set to now.
func New() *Collector {
	return &Collector{startTime: time.Now()}
}

// ── Connection metrics ───────────────────────────────────────────────

// ConnectionOpened increments both the active and total counters.
func (c *Collector) ConnectionOpened() {
	if c == nil {
		return
	}
	c.connectionsActive.Add(1)
	c.connectionsTotal.Add(1)
}

// ConnectionClosed decrements the active connection counter.
func (c *Collector) ConnectionClosed() {
	if c == nil {
		return
	}
	c.connectionsActive.Add(-1)
}

// ActiveConnections returns the current number of open connections.
func (c *Collector) ActiveConnections() int64 {
	if c == nil {
		return 0
	}
	return c.connectionsActive.Load()
}

// TotalConnections returns the lifetime connection count.
func (c *Collector) TotalConnections() int64 {
	if c == nil {
		return 0
	}
	return c.connectionsTotal.Load()
}

// ── I/O metrics ──────────────────────────────────────────────────────

// BytesReceived records n bytes read from the network.
func (c *Collector) BytesReceived(n int64) {
	if c == nil {
		return
	}
	c.bytesIn.Add(n)
}

// BytesSent records n bytes written to the network.
func (c *Collector) BytesSent(n int64) {
	if c == nil {
		return
	}
	c.bytesOut.Add(n)
}

// TotalBytesIn returns total bytes received.
func (c *Collector) TotalBytesIn() int64 {
	if c == nil {
		return 0
	}
	return c.bytesIn.Load()
}

// TotalBytesOut returns total bytes sent.
func (c *Collector) TotalBytesOut() int64 {
	if c == nil {
		return 0
	}
	return c.bytesOut.Load()
}

// ── Tunnel metrics ───────────────────────────────────────────────────

// TunnelReconnect records a tunnel reconnection event.
func (c *Collector) TunnelReconnect() {
	if c == nil {
		return
	}
	c.tunnelReconnects.Add(1)
}

// TunnelReconnects returns the total tunnel reconnection count.
func (c *Collector) TunnelReconnects() int64 {
	if c == nil {
		return 0
	}
	return c.tunnelReconnects.Load()
}

// ── Error metrics ────────────────────────────────────────────────────

// RecordError increments the error counter and stores the message.
func (c *Collector) RecordError(msg string) {
	if c == nil {
		return
	}
	c.errorsTotal.Add(1)
	c.mu.Lock()
	c.lastError = time.Now()
	c.lastErrorMsg = msg
	c.mu.Unlock()
}

// ErrorCount returns the total number of errors recorded.
func (c *Collector) ErrorCount() int64 {
	if c == nil {
		return 0
	}
	return c.errorsTotal.Load()
}

// ── Health ───────────────────────────────────────────────────────────

// RecordHealthCheck updates the last health check timestamp.
func (c *Collector) RecordHealthCheck() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.lastHealthCheck = time.Now()
	c.mu.Unlock()
}

// ── Snapshot ─────────────────────────────────────────────────────────

// Snapshot is a point-in-time view of all metrics.
type Snapshot struct {
	Uptime            string `json:"uptime"`
	ConnectionsActive int64  `json:"connections_active"`
	ConnectionsTotal  int64  `json:"connections_total"`
	BytesIn           int64  `json:"bytes_in"`
	BytesOut          int64  `json:"bytes_out"`
	TunnelReconnects  int64  `json:"tunnel_reconnects"`
	ErrorsTotal       int64  `json:"errors_total"`
	LastHealthCheck   string `json:"last_health_check,omitempty"`
	LastError         string `json:"last_error,omitempty"`
	LastErrorMessage  string `json:"last_error_message,omitempty"`
}

// Snapshot returns a copy of all current metrics.
func (c *Collector) Snapshot() Snapshot {
	if c == nil {
		return Snapshot{}
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	s := Snapshot{
		Uptime:            time.Since(c.startTime).Truncate(time.Second).String(),
		ConnectionsActive: c.connectionsActive.Load(),
		ConnectionsTotal:  c.connectionsTotal.Load(),
		BytesIn:           c.bytesIn.Load(),
		BytesOut:          c.bytesOut.Load(),
		TunnelReconnects:  c.tunnelReconnects.Load(),
		ErrorsTotal:       c.errorsTotal.Load(),
	}
	if !c.lastHealthCheck.IsZero() {
		s.LastHealthCheck = c.lastHealthCheck.Format(time.RFC3339)
	}
	if !c.lastError.IsZero() {
		s.LastError = c.lastError.Format(time.RFC3339)
		s.LastErrorMessage = c.lastErrorMsg
	}
	return s
}

// JSON returns the snapshot as an indented JSON string.
func (c *Collector) JSON() string {
	s := c.Snapshot()
	data, _ := json.MarshalIndent(s, "", "  ")
	return string(data)
}

package metrics

import (
	"encoding/json"
	"testing"
)

func TestCollector_Connections(t *testing.T) {
	c := New()

	c.ConnectionOpened()
	c.ConnectionOpened()
	if c.ActiveConnections() != 2 {
		t.Errorf("active = %d, want 2", c.ActiveConnections())
	}
	if c.TotalConnections() != 2 {
		t.Errorf("total = %d, want 2", c.TotalConnections())
	}

	c.ConnectionClosed()
	if c.ActiveConnections() != 1 {
		t.Errorf("active = %d, want 1", c.ActiveConnections())
	}
	if c.TotalConnections() != 2 {
		t.Errorf("total should remain 2, got %d", c.TotalConnections())
	}
}

func TestCollector_Bytes(t *testing.T) {
	c := New()

	c.BytesReceived(1024)
	c.BytesSent(512)
	c.BytesReceived(100)

	if c.TotalBytesIn() != 1124 {
		t.Errorf("bytes in = %d, want 1124", c.TotalBytesIn())
	}
	if c.TotalBytesOut() != 512 {
		t.Errorf("bytes out = %d, want 512", c.TotalBytesOut())
	}
}

func TestCollector_TunnelReconnects(t *testing.T) {
	c := New()

	c.TunnelReconnect()
	c.TunnelReconnect()
	c.TunnelReconnect()

	if c.TunnelReconnects() != 3 {
		t.Errorf("reconnects = %d, want 3", c.TunnelReconnects())
	}
}

func TestCollector_Errors(t *testing.T) {
	c := New()

	c.RecordError("first error")
	c.RecordError("second error")

	if c.ErrorCount() != 2 {
		t.Errorf("errors = %d, want 2", c.ErrorCount())
	}
}

func TestCollector_HealthCheck(t *testing.T) {
	c := New()
	c.RecordHealthCheck()

	snap := c.Snapshot()
	if snap.LastHealthCheck == "" {
		t.Error("expected non-empty health check timestamp")
	}
}

func TestCollector_Snapshot(t *testing.T) {
	c := New()
	c.ConnectionOpened()
	c.BytesReceived(100)
	c.BytesSent(50)
	c.RecordError("test")

	snap := c.Snapshot()
	if snap.ConnectionsActive != 1 {
		t.Errorf("snap active = %d", snap.ConnectionsActive)
	}
	if snap.BytesIn != 100 {
		t.Errorf("snap bytes in = %d", snap.BytesIn)
	}
	if snap.ErrorsTotal != 1 {
		t.Errorf("snap errors = %d", snap.ErrorsTotal)
	}
	if snap.LastErrorMessage != "test" {
		t.Errorf("snap error msg = %q", snap.LastErrorMessage)
	}
}

func TestCollector_JSON(t *testing.T) {
	c := New()
	c.ConnectionOpened()
	c.BytesSent(42)

	raw := c.JSON()
	var snap Snapshot
	if err := json.Unmarshal([]byte(raw), &snap); err != nil {
		t.Fatalf("JSON parse error: %v", err)
	}
	if snap.ConnectionsActive != 1 {
		t.Errorf("JSON active = %d", snap.ConnectionsActive)
	}
	if snap.BytesOut != 42 {
		t.Errorf("JSON bytes out = %d", snap.BytesOut)
	}
}

func TestNilCollector_NoOps(t *testing.T) {
	var c *Collector

	// None of these should panic.
	c.ConnectionOpened()
	c.ConnectionClosed()
	c.BytesReceived(100)
	c.BytesSent(100)
	c.TunnelReconnect()
	c.RecordError("test")
	c.RecordHealthCheck()

	if c.ActiveConnections() != 0 {
		t.Error("nil collector should return 0")
	}
	if c.TotalBytesIn() != 0 {
		t.Error("nil collector should return 0")
	}
	if c.ErrorCount() != 0 {
		t.Error("nil collector should return 0")
	}

	snap := c.Snapshot()
	if snap.ConnectionsActive != 0 {
		t.Error("nil snapshot should be zero")
	}

	j := c.JSON()
	if j == "" {
		t.Error("nil JSON should return valid JSON")
	}
}

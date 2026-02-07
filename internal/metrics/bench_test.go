package metrics

import "testing"

// BenchmarkCollector_ConnectionOpen measures the overhead of recording
// a connection open event (atomic operations).
func BenchmarkCollector_ConnectionOpen(b *testing.B) {
	c := New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.ConnectionOpened()
	}
}

// BenchmarkCollector_BytesSent measures byte-counter overhead.
func BenchmarkCollector_BytesSent(b *testing.B) {
	c := New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.BytesSent(32768)
	}
}

// BenchmarkCollector_Snapshot measures the cost of taking a snapshot.
func BenchmarkCollector_Snapshot(b *testing.B) {
	c := New()
	c.ConnectionOpened()
	c.BytesSent(1024)
	c.RecordError("test")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Snapshot()
	}
}

// BenchmarkCollector_JSON measures JSON export overhead.
func BenchmarkCollector_JSON(b *testing.B) {
	c := New()
	c.ConnectionOpened()
	c.BytesSent(1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.JSON()
	}
}

// BenchmarkNilCollector verifies nil-safe no-ops have zero overhead.
func BenchmarkNilCollector(b *testing.B) {
	var c *Collector
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.ConnectionOpened()
		c.BytesSent(32768)
		c.RecordError("test")
	}
}

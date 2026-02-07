package retry

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// BenchmarkBackoff_ImmediateSuccess measures overhead when the first
// attempt succeeds (the common case).
func BenchmarkBackoff_ImmediateSuccess(b *testing.B) {
	bo := DefaultBackoff()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bo.Do(ctx, func(_ int) error { return nil }) //nolint:errcheck
	}
}

// BenchmarkBackoff_PermanentError measures early-exit overhead.
func BenchmarkBackoff_PermanentError(b *testing.B) {
	bo := DefaultBackoff()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bo.Do(ctx, func(_ int) error { //nolint:errcheck
			return Permanent(fmt.Errorf("fatal"))
		})
	}
}

// BenchmarkCircuitBreaker_ClosedPath benchmarks the fast path when the
// circuit is closed and the operation succeeds.
func BenchmarkCircuitBreaker_ClosedPath(b *testing.B) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Execute(func() error { return nil }) //nolint:errcheck
	}
}

// BenchmarkCircuitBreaker_OpenPath benchmarks rejection when open.
func BenchmarkCircuitBreaker_OpenPath(b *testing.B) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		MaxFailures:  1,
		ResetTimeout: time.Hour,
		HalfOpenMax:  1,
	})
	// Trip the circuit.
	cb.Execute(func() error { return fmt.Errorf("fail") }) //nolint:errcheck

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Execute(func() error { return nil }) //nolint:errcheck
	}
}

// BenchmarkJitter measures the jitter helper without backoff overhead.
func BenchmarkJitter(b *testing.B) {
	d := 100 * time.Millisecond
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = addJitter(d)
	}
}

// Package retry provides exponential backoff and circuit breaker
// patterns for resilient network operations.
package retry

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// ── Permanent errors ─────────────────────────────────────────────────

// PermanentError wraps an error to signal that retrying will not help.
// Return [Permanent](err) from the operation function to stop retrying
// immediately.
type PermanentError struct {
	Err error
}

func (e *PermanentError) Error() string { return e.Err.Error() }
func (e *PermanentError) Unwrap() error { return e.Err }

// Permanent marks err as non-retryable.  The backoff loop will return
// the inner error immediately without further attempts.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &PermanentError{Err: err}
}

// IsPermanent reports whether err has been marked as permanent.
func IsPermanent(err error) bool {
	var pe *PermanentError
	return errors.As(err, &pe)
}

// ── Backoff ──────────────────────────────────────────────────────────

// Backoff implements exponential backoff with optional jitter.
type Backoff struct {
	// InitialDelay is the delay before the first retry (default 1s).
	InitialDelay time.Duration
	// MaxDelay caps the backoff duration (default 60s).
	MaxDelay time.Duration
	// Multiplier increases the delay each attempt (default 2.0).
	Multiplier float64
	// MaxAttempts is the total number of tries including the first.
	// Set to 0 for unlimited retries (until context cancelled).
	// Default: 10.
	MaxAttempts int
	// Jitter adds ±25% randomisation to prevent thundering herd.
	Jitter bool
}

// DefaultBackoff returns a reasonable default configuration.
func DefaultBackoff() *Backoff {
	return &Backoff{
		InitialDelay: 1 * time.Second,
		MaxDelay:     60 * time.Second,
		Multiplier:   2.0,
		MaxAttempts:  10,
		Jitter:       true,
	}
}

// Do executes fn repeatedly until it succeeds, returns a permanent
// error, or the retry budget (attempts / context) is exhausted.
//
// The attempt parameter passed to fn is 1-based.  On success fn should
// return nil.  To abort retrying, wrap the error with [Permanent].
func (b *Backoff) Do(ctx context.Context, fn func(attempt int) error) error {
	delay := b.InitialDelay
	if delay == 0 {
		delay = time.Second
	}
	multiplier := b.Multiplier
	if multiplier <= 0 {
		multiplier = 2.0
	}
	maxDelay := b.MaxDelay
	if maxDelay == 0 {
		maxDelay = 60 * time.Second
	}

	for attempt := 1; ; attempt++ {
		err := fn(attempt)
		if err == nil {
			return nil
		}

		// Permanent errors are never retried.
		if IsPermanent(err) {
			return errors.Unwrap(err)
		}

		// Check attempt budget.
		if b.MaxAttempts > 0 && attempt >= b.MaxAttempts {
			return fmt.Errorf("max retries (%d) exceeded: %w", b.MaxAttempts, err)
		}

		// Apply jitter.
		wait := delay
		if b.Jitter {
			wait = addJitter(delay)
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		case <-time.After(wait):
		}

		// Increase delay for next iteration.
		delay = time.Duration(float64(delay) * multiplier)
		if delay > maxDelay {
			delay = maxDelay
		}
	}
}

// addJitter adds ±25% randomisation to a duration.
func addJitter(d time.Duration) time.Duration {
	quarter := float64(d) * 0.25
	delta := (rand.Float64() * 2 * quarter) - quarter
	result := float64(d) + delta
	return time.Duration(math.Max(result, float64(time.Millisecond)))
}

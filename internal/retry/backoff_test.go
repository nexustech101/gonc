package retry

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestBackoff_SuccessAfterRetries(t *testing.T) {
	b := &Backoff{
		InitialDelay: time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   1.5,
		MaxAttempts:  10,
	}
	calls := 0

	err := b.Do(context.Background(), func(attempt int) error {
		calls++
		if attempt < 3 {
			return fmt.Errorf("transient")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestBackoff_ImmediateSuccess(t *testing.T) {
	b := DefaultBackoff()

	err := b.Do(context.Background(), func(_ int) error {
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBackoff_PermanentError(t *testing.T) {
	b := DefaultBackoff()
	calls := 0

	err := b.Do(context.Background(), func(_ int) error {
		calls++
		return Permanent(fmt.Errorf("fatal"))
	})

	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "fatal" {
		t.Errorf("expected 'fatal', got %q", err.Error())
	}
	if calls != 1 {
		t.Errorf("permanent error should stop after 1 call, got %d", calls)
	}
}

func TestBackoff_MaxAttempts(t *testing.T) {
	b := &Backoff{
		InitialDelay: time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   1.5,
		MaxAttempts:  3,
	}
	calls := 0

	err := b.Do(context.Background(), func(_ int) error {
		calls++
		return fmt.Errorf("always fails")
	})

	if err == nil {
		t.Fatal("expected error after max attempts")
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestBackoff_ContextCancelled(t *testing.T) {
	b := &Backoff{
		InitialDelay: 5 * time.Second,
		MaxAttempts:  100,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := b.Do(ctx, func(_ int) error {
		return fmt.Errorf("fail")
	})

	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestPermanent_Nil(t *testing.T) {
	if Permanent(nil) != nil {
		t.Error("Permanent(nil) should be nil")
	}
}

func TestIsPermanent(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"permanent", Permanent(fmt.Errorf("x")), true},
		{"not permanent", fmt.Errorf("x"), false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPermanent(tt.err); got != tt.want {
				t.Errorf("IsPermanent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJitter_Range(t *testing.T) {
	d := 100 * time.Millisecond
	for i := 0; i < 100; i++ {
		j := addJitter(d)
		lower := time.Duration(float64(d) * 0.74)
		upper := time.Duration(float64(d) * 1.26)
		if j < lower || j > upper {
			t.Errorf("jitter %v out of expected range [%v, %v]", j, lower, upper)
		}
	}
}

func TestBackoff_ZeroConfig(t *testing.T) {
	// Zero-value Backoff should use sensible defaults internally.
	b := &Backoff{MaxAttempts: 2}
	calls := 0

	start := time.Now()
	_ = b.Do(context.Background(), func(_ int) error {
		calls++
		return fmt.Errorf("fail")
	})
	elapsed := time.Since(start)

	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
	// Should have waited ~1s (the default initial delay).
	if elapsed < 500*time.Millisecond {
		t.Errorf("expected at least 500ms delay, got %v", elapsed)
	}
}

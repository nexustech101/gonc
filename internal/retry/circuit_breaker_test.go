package retry

import (
	"fmt"
	"testing"
	"time"
)

func TestCircuitBreaker_NormalOperation(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())

	err := cb.Execute(func() error { return nil })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cb.CurrentState() != StateClosed {
		t.Errorf("expected closed, got %s", cb.CurrentState())
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		MaxFailures:  3,
		ResetTimeout: time.Second,
		HalfOpenMax:  1,
	})

	for i := 0; i < 3; i++ {
		cb.Execute(func() error { return fmt.Errorf("fail") }) //nolint:errcheck
	}

	if cb.CurrentState() != StateOpen {
		t.Errorf("expected open after 3 failures, got %s", cb.CurrentState())
	}
	if cb.Failures() != 3 {
		t.Errorf("expected 3 failures, got %d", cb.Failures())
	}
}

func TestCircuitBreaker_RejectsWhenOpen(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		MaxFailures:  1,
		ResetTimeout: time.Hour, // long timeout so it stays open
		HalfOpenMax:  1,
	})

	// Trip the breaker.
	cb.Execute(func() error { return fmt.Errorf("fail") }) //nolint:errcheck

	// Should be rejected without calling fn.
	called := false
	err := cb.Execute(func() error {
		called = true
		return nil
	})

	if err == nil {
		t.Fatal("expected error when circuit is open")
	}
	if called {
		t.Error("fn should not have been called when circuit is open")
	}
}

func TestCircuitBreaker_HalfOpenRecovery(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		MaxFailures:  1,
		ResetTimeout: 10 * time.Millisecond,
		HalfOpenMax:  2,
	})

	// Trip the breaker.
	cb.Execute(func() error { return fmt.Errorf("fail") }) //nolint:errcheck
	if cb.CurrentState() != StateOpen {
		t.Fatalf("expected open, got %s", cb.CurrentState())
	}

	// Wait for reset timeout.
	time.Sleep(20 * time.Millisecond)

	// First success moves to half-open, but 2 required to close.
	err := cb.Execute(func() error { return nil })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cb.CurrentState() != StateHalfOpen {
		t.Errorf("expected half-open after first success, got %s", cb.CurrentState())
	}

	// Second success closes the circuit.
	err = cb.Execute(func() error { return nil })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cb.CurrentState() != StateClosed {
		t.Errorf("expected closed after 2 successes, got %s", cb.CurrentState())
	}
}

func TestCircuitBreaker_HalfOpenFailure(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		MaxFailures:  1,
		ResetTimeout: 10 * time.Millisecond,
		HalfOpenMax:  2,
	})

	// Trip → Open.
	cb.Execute(func() error { return fmt.Errorf("fail") }) //nolint:errcheck
	time.Sleep(20 * time.Millisecond)

	// Half-open probe fails → back to Open.
	cb.Execute(func() error { return fmt.Errorf("still broken") }) //nolint:errcheck
	if cb.CurrentState() != StateOpen {
		t.Errorf("expected open after half-open failure, got %s", cb.CurrentState())
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		MaxFailures:  1,
		ResetTimeout: time.Hour,
		HalfOpenMax:  1,
	})

	cb.Execute(func() error { return fmt.Errorf("fail") }) //nolint:errcheck
	if cb.CurrentState() != StateOpen {
		t.Fatalf("expected open, got %s", cb.CurrentState())
	}

	cb.Reset()
	if cb.CurrentState() != StateClosed {
		t.Errorf("expected closed after reset, got %s", cb.CurrentState())
	}
	if cb.Failures() != 0 {
		t.Errorf("expected 0 failures after reset, got %d", cb.Failures())
	}
}

func TestCircuitBreaker_StateChange(t *testing.T) {
	var transitions []string
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		MaxFailures:  1,
		ResetTimeout: 10 * time.Millisecond,
		HalfOpenMax:  1,
		OnStateChange: func(from, to State) {
			transitions = append(transitions, fmt.Sprintf("%s→%s", from, to))
		},
	})

	cb.Execute(func() error { return fmt.Errorf("fail") }) //nolint:errcheck
	time.Sleep(20 * time.Millisecond)
	cb.Execute(func() error { return nil }) //nolint:errcheck

	want := []string{"closed→open", "open→half-open", "half-open→closed"}
	if len(transitions) != len(want) {
		t.Fatalf("transitions = %v, want %v", transitions, want)
	}
	for i := range want {
		if transitions[i] != want[i] {
			t.Errorf("transition[%d] = %q, want %q", i, transitions[i], want[i])
		}
	}
}

func TestState_String(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{State(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("State(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestCircuitBreaker_NilConfig(t *testing.T) {
	cb := NewCircuitBreaker(nil)
	if cb.maxFailures != 5 {
		t.Errorf("expected default maxFailures=5, got %d", cb.maxFailures)
	}
}

func TestCircuitBreaker_SuccessResetsFailureCount(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		MaxFailures:  3,
		ResetTimeout: time.Second,
		HalfOpenMax:  1,
	})

	// 2 failures, then success → failures reset.
	cb.Execute(func() error { return fmt.Errorf("fail") }) //nolint:errcheck
	cb.Execute(func() error { return fmt.Errorf("fail") }) //nolint:errcheck
	cb.Execute(func() error { return nil })                 //nolint:errcheck

	if cb.Failures() != 0 {
		t.Errorf("expected 0 failures after success, got %d", cb.Failures())
	}
	if cb.CurrentState() != StateClosed {
		t.Errorf("expected closed, got %s", cb.CurrentState())
	}
}

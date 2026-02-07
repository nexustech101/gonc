package retry

import (
	"fmt"
	"sync"
	"time"
)

// ── Circuit breaker state ────────────────────────────────────────────

// State represents the circuit breaker's operational state.
type State int

const (
	// StateClosed is normal operation — requests pass through.
	StateClosed State = iota
	// StateOpen means the service is failing — requests are rejected.
	StateOpen
	// StateHalfOpen allows a limited number of probes to test recovery.
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// ── Configuration ────────────────────────────────────────────────────

// CircuitBreakerConfig configures a [CircuitBreaker].
type CircuitBreakerConfig struct {
	// MaxFailures is the number of consecutive failures before opening
	// the circuit (default 5).
	MaxFailures int
	// ResetTimeout is how long the circuit stays open before moving to
	// half-open (default 30s).
	ResetTimeout time.Duration
	// HalfOpenMax is the number of consecutive successes in half-open
	// state required to close the circuit (default 2).
	HalfOpenMax int
	// OnStateChange is called whenever the state transitions.  It runs
	// under the lock, so keep it fast.
	OnStateChange func(from, to State)
}

// DefaultCircuitBreakerConfig returns sensible defaults.
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		MaxFailures:  5,
		ResetTimeout: 30 * time.Second,
		HalfOpenMax:  2,
	}
}

// ── CircuitBreaker ───────────────────────────────────────────────────

// CircuitBreaker prevents repeated calls to a failing service by
// tracking consecutive failures and short-circuiting when a threshold
// is crossed.
type CircuitBreaker struct {
	mu            sync.Mutex
	state         State
	failures      int
	successes     int
	maxFailures   int
	resetTimeout  time.Duration
	halfOpenMax   int
	lastFailure   time.Time
	onStateChange func(from, to State)
}

// NewCircuitBreaker creates a circuit breaker with the given config.
func NewCircuitBreaker(cfg *CircuitBreakerConfig) *CircuitBreaker {
	if cfg == nil {
		cfg = DefaultCircuitBreakerConfig()
	}
	maxF := cfg.MaxFailures
	if maxF <= 0 {
		maxF = 5
	}
	rt := cfg.ResetTimeout
	if rt <= 0 {
		rt = 30 * time.Second
	}
	hom := cfg.HalfOpenMax
	if hom <= 0 {
		hom = 2
	}
	return &CircuitBreaker{
		state:         StateClosed,
		maxFailures:   maxF,
		resetTimeout:  rt,
		halfOpenMax:   hom,
		onStateChange: cfg.OnStateChange,
	}
}

// Execute runs fn through the circuit breaker.  When the circuit is
// open, fn is not called and an error is returned immediately.
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if err := cb.beforeRequest(); err != nil {
		return err
	}
	err := fn()
	cb.afterRequest(err)
	return err
}

// CurrentState returns the current circuit breaker state.
func (cb *CircuitBreaker) CurrentState() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// Failures returns the current consecutive failure count.
func (cb *CircuitBreaker) Failures() int {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.failures
}

// Reset forces the circuit breaker back to closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
	cb.successes = 0
	cb.transition(StateClosed)
}

// ── internal ─────────────────────────────────────────────────────────

func (cb *CircuitBreaker) beforeRequest() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return nil
	case StateOpen:
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			cb.transition(StateHalfOpen)
			return nil
		}
		remaining := cb.resetTimeout - time.Since(cb.lastFailure)
		return fmt.Errorf("circuit open: %d consecutive failures, retry in %v",
			cb.failures, remaining.Truncate(time.Second))
	case StateHalfOpen:
		return nil
	}
	return nil
}

func (cb *CircuitBreaker) afterRequest(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failures++
		cb.successes = 0
		cb.lastFailure = time.Now()

		if cb.state == StateHalfOpen || cb.failures >= cb.maxFailures {
			cb.transition(StateOpen)
		}
	} else {
		cb.successes++

		switch cb.state {
		case StateHalfOpen:
			if cb.successes >= cb.halfOpenMax {
				cb.failures = 0
				cb.transition(StateClosed)
			}
		case StateClosed:
			cb.failures = 0
		}
	}
}

func (cb *CircuitBreaker) transition(to State) {
	from := cb.state
	if from == to {
		return
	}
	cb.state = to
	if cb.onStateChange != nil {
		cb.onStateChange(from, to)
	}
}

package tunnel

import (
	"context"
	"sync"
	"time"

	"gonc/util"
)

// Manager wraps an SSHTunnel and adds periodic health monitoring.
type Manager struct {
	tunnel  *SSHTunnel
	logger  *util.Logger
	mu      sync.RWMutex
	stopped bool
}

// NewManager returns a Manager for the given tunnel.
func NewManager(t *SSHTunnel, logger *util.Logger) *Manager {
	return &Manager{tunnel: t, logger: logger}
}

// Start connects the tunnel and begins background health checks.
func (m *Manager) Start(ctx context.Context) error {
	if err := m.tunnel.Connect(ctx); err != nil {
		return err
	}
	go m.healthLoop(ctx)
	return nil
}

// Stop gracefully shuts down the tunnel.
func (m *Manager) Stop() error {
	m.mu.Lock()
	m.stopped = true
	m.mu.Unlock()
	return m.tunnel.Close()
}

func (m *Manager) healthLoop(ctx context.Context) {
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			m.mu.RLock()
			done := m.stopped
			m.mu.RUnlock()
			if done {
				return
			}
			if !m.tunnel.IsAlive() {
				m.logger.Error("SSH tunnel connection lost")
				return
			}
		}
	}
}

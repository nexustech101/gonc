// Package capability defines what happens over an established
// connection.  Each Capability encapsulates a single behaviour
// (relay I/O, execute a program, etc.) and operates on a Session
// rather than a raw net.Conn, which keeps capabilities testable
// and decoupled from transport details.
package capability

import (
	"context"

	"gonc/internal/session"
)

// Capability handles a single connection according to a specific
// behaviour.  Implementations include relaying stdin/stdout (Relay)
// and executing a child process (Exec).
type Capability interface {
	// Handle runs the capability against the given session.
	// It blocks until the connection is done or the context is
	// cancelled.
	Handle(ctx context.Context, sess *session.Session) error
}

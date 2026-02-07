package capability

import (
	"context"

	"gonc/internal/session"
	"gonc/util"
)

// Relay copies data bidirectionally between the connection and the
// session's stdin/stdout — the default interactive / pipe mode.
type Relay struct{}

// Handle shuttles bytes between the network connection and the local
// I/O endpoints until one side closes or the context is cancelled.
func (r *Relay) Handle(ctx context.Context, sess *session.Session) error {
	return util.BidirectionalCopy(ctx, sess.Conn, sess.Stdin, sess.Stdout)
}

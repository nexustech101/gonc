// Package session represents a single connection lifecycle, binding a
// network connection with I/O endpoints and shared context.
//
// Sessions decouple capabilities from concrete I/O sources — a
// capability doesn't need to know whether it's reading from os.Stdin
// or a test buffer, it just uses the session's Reader/Writer.
package session

import (
	"io"
	"net"

	"gonc/util"
)

// Session encapsulates the runtime context for a single connection.
// Capabilities operate on sessions rather than raw connections,
// enabling clean testing and I/O abstraction.
type Session struct {
	Conn   net.Conn
	Stdin  io.Reader
	Stdout io.Writer
	Logger *util.Logger
}

// New creates a Session bound to the given connection and I/O pair.
func New(conn net.Conn, stdin io.Reader, stdout io.Writer, logger *util.Logger) *Session {
	return &Session{
		Conn:   conn,
		Stdin:  stdin,
		Stdout: stdout,
		Logger: logger,
	}
}

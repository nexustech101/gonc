// Package core is the orchestration layer.  It composes transports
// and capabilities into complete operational modes and provides a
// builder that selects the right mode from a Config.
//
// Architecture layers (bottom → top):
//
//	transport  →  capability  →  session  →  core  →  cmd (CLI)
//
// The builder in this package is the single dispatch point that
// replaces the scattered switch/if trees that previously lived in
// netcat.Run() and cmd.Execute().
package core

import "context"

// Mode represents a complete operational mode of gonc (connect,
// listen, scan, or reverse-tunnel).  Each mode owns its full
// lifecycle from connection establishment to teardown.
type Mode interface {
	Run(ctx context.Context) error
}

// Package util provides low-level helpers shared by all other packages.
package util

import (
	"fmt"
	"io"
	"os"
	"sync"
)

// LogLevel controls output verbosity.
type LogLevel int

const (
	LogQuiet   LogLevel = 0
	LogNormal  LogLevel = 1
	LogVerbose LogLevel = 2
	LogDebug   LogLevel = 3
)

// Logger writes levelled messages to stderr.
type Logger struct {
	level  LogLevel
	output io.Writer
	mu     sync.Mutex
}

// NewLogger returns a Logger that prints messages at or below the given
// verbosity (0 = quiet, 1 = normal, 2 = verbose, 3 = debug).
func NewLogger(verbosity int) *Logger {
	return &Logger{
		level:  LogLevel(verbosity),
		output: os.Stderr,
	}
}

// Info prints when verbosity ≥ 1.
func (l *Logger) Info(format string, args ...interface{}) {
	if l.level >= LogNormal {
		l.write(format, args...)
	}
}

// Verbose prints when verbosity ≥ 2.
func (l *Logger) Verbose(format string, args ...interface{}) {
	if l.level >= LogVerbose {
		l.write(format, args...)
	}
}

// Debug prints when verbosity ≥ 3.
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.level >= LogDebug {
		l.write(format, args...)
	}
}

// Error always prints regardless of verbosity.
func (l *Logger) Error(format string, args ...interface{}) {
	l.write(format, args...)
}

func (l *Logger) write(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(l.output, format+"\n", args...)
}

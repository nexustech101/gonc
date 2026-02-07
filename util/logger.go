// Package util provides low-level helpers shared by all other packages.
package util

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// LogLevel controls output verbosity.
type LogLevel int

const (
	LogQuiet   LogLevel = 0
	LogNormal  LogLevel = 1
	LogVerbose LogLevel = 2
	LogDebug   LogLevel = 3
)

// Logger writes levelled messages to stderr with optional timestamps
// and level prefixes.
type Logger struct {
	level      LogLevel
	output     io.Writer
	mu         sync.Mutex
	timestamps bool // if true, prepend RFC3339 timestamps
}

// NewLogger returns a Logger that prints messages at or below the given
// verbosity (0 = quiet, 1 = normal, 2 = verbose, 3 = debug).
func NewLogger(verbosity int) *Logger {
	return &Logger{
		level:      LogLevel(verbosity),
		output:     os.Stderr,
		timestamps: verbosity >= 3, // auto-enable timestamps in debug mode
	}
}

// SetTimestamps enables or disables timestamp prefixes.
func (l *Logger) SetTimestamps(on bool) { l.timestamps = on }

// SetOutput overrides the output writer (default: os.Stderr).
func (l *Logger) SetOutput(w io.Writer) { l.output = w }

// Level returns the current log level.
func (l *Logger) Level() LogLevel { return l.level }

// Info prints when verbosity ≥ 1.  Prefixed with [INF].
func (l *Logger) Info(format string, args ...interface{}) {
	if l.level >= LogNormal {
		l.write("INF", format, args...)
	}
}

// Warn prints when verbosity ≥ 1.  Prefixed with [WRN].
func (l *Logger) Warn(format string, args ...interface{}) {
	if l.level >= LogNormal {
		l.write("WRN", format, args...)
	}
}

// Verbose prints when verbosity ≥ 2.  Prefixed with [VRB].
func (l *Logger) Verbose(format string, args ...interface{}) {
	if l.level >= LogVerbose {
		l.write("VRB", format, args...)
	}
}

// Debug prints when verbosity ≥ 3.  Prefixed with [DBG].
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.level >= LogDebug {
		l.write("DBG", format, args...)
	}
}

// Error always prints regardless of verbosity.  Prefixed with [ERR].
func (l *Logger) Error(format string, args ...interface{}) {
	l.write("ERR", format, args...)
}

func (l *Logger) write(level, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	msg := fmt.Sprintf(format, args...)
	if l.timestamps {
		ts := time.Now().Format("15:04:05.000")
		fmt.Fprintf(l.output, "%s [%s] %s\n", ts, level, msg)
	} else {
		fmt.Fprintf(l.output, "[%s] %s\n", level, msg)
	}
}

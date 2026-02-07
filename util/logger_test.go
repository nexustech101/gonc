package util

import (
	"bytes"
	"strings"
	"testing"
)

func TestLogger_Levels(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(3) // debug level
	l.SetOutput(&buf)
	l.SetTimestamps(false)

	l.Error("e")
	l.Warn("w")
	l.Info("i")
	l.Verbose("v")
	l.Debug("d")

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d:\n%s", len(lines), output)
	}

	wantPrefixes := []string{"[ERR]", "[WRN]", "[INF]", "[VRB]", "[DBG]"}
	for i, prefix := range wantPrefixes {
		if !strings.Contains(lines[i], prefix) {
			t.Errorf("line %d %q missing prefix %q", i, lines[i], prefix)
		}
	}
}

func TestLogger_QuietMode(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(0) // quiet
	l.SetOutput(&buf)
	l.SetTimestamps(false)

	l.Info("should not appear")
	l.Verbose("should not appear")
	l.Debug("should not appear")
	l.Error("always appears")

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 line in quiet mode, got %d:\n%s", len(lines), output)
	}
}

func TestLogger_Timestamps(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(1)
	l.SetOutput(&buf)
	l.SetTimestamps(true)

	l.Info("test")

	output := buf.String()
	// Timestamp format is "HH:MM:SS.mmm"
	if !strings.Contains(output, ":") || len(output) < 15 {
		t.Errorf("expected timestamp prefix, got %q", output)
	}
}

func TestLogger_WarnLevel(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(1) // normal
	l.SetOutput(&buf)
	l.SetTimestamps(false)

	l.Warn("warning message")

	if !strings.Contains(buf.String(), "[WRN]") {
		t.Errorf("expected [WRN] prefix, got %q", buf.String())
	}
}

func TestBufPool_RoundTrip(t *testing.T) {
	buf := GetBuf()
	if buf == nil {
		t.Fatal("GetBuf returned nil")
	}
	if len(*buf) != DefaultBufSize {
		t.Errorf("buffer size = %d, want %d", len(*buf), DefaultBufSize)
	}

	// Write some data and return.
	(*buf)[0] = 0xFF
	PutBuf(buf)

	// Get another buffer — may or may not be the same one.
	buf2 := GetBuf()
	if buf2 == nil {
		t.Fatal("second GetBuf returned nil")
	}
	PutBuf(buf2)
}

func TestPutBuf_Nil(t *testing.T) {
	// Should not panic.
	PutBuf(nil)
}

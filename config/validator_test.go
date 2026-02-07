package config

import (
	"strings"
	"testing"
)

// TestValidate_ErrorMessages verifies that Validate returns actionable
// error messages with hints.
func TestValidate_ErrorMessages(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantSub string // substring expected in error
	}{
		{
			name:    "listen no port has hint",
			cfg:     Config{Listen: true},
			wantSub: "hint:",
		},
		{
			name:    "reverse tunnel no remote-port has hint",
			cfg:     Config{Listen: true, LocalPort: 8080, ReverseTunnelEnabled: true, ReverseTunnelHost: "gw"},
			wantSub: "hint:",
		},
		{
			name:    "exec conflict",
			cfg:     Config{Host: "x", Port: 80, Execute: "a", Command: "b"},
			wantSub: "-e and -c are mutually exclusive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("error %q should contain %q", err.Error(), tt.wantSub)
			}
		})
	}
}

// TestParsePortSpec_Fuzz covers edge-case port specs.
func TestParsePortSpec_Fuzz(t *testing.T) {
	edgeCases := []string{
		"1", "65535", "1-1", "1-65535",
		"-1", "65536", "abc-def", "-", "1-", "-1",
		"0", "99999", "1-0",
	}
	for _, s := range edgeCases {
		t.Run(s, func(t *testing.T) {
			pr, err := ParsePortSpec(s)
			if err == nil {
				// Valid result: check invariants.
				if pr.Start < 1 || pr.End > 65535 || pr.Start > pr.End {
					t.Errorf("invalid range: %+v", pr)
				}
			}
			// Invalid specs just return errors, which is fine.
		})
	}
}

// TestParseTunnelSpec_EdgeCases covers additional tunnel specs.
func TestParseTunnelSpec_EdgeCases(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"user@host.with.dots:22", false},
		{"user@host-with-dashes", false},
		{"host:0", true},           // port 0 out of range
		{"host:65536", true},       // port too high
		{"user@", false},           // regex treats "user@" as hostname
		{"", true},                 // empty string
		{":22", true},              // no host before colon
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, _, _, err := ParseTunnelSpec(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTunnelSpec(%q) err = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

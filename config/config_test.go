package config

import (
	"testing"
)

// ── ParseTunnelSpec ──────────────────────────────────────────────────

func TestParseTunnelSpec(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantUser string
		wantHost string
		wantPort int
		wantErr  bool
	}{
		{"full", "admin@bastion.example.com:2222", "admin", "bastion.example.com", 2222, false},
		{"no port", "root@gateway", "root", "gateway", 22, false},
		{"no user", "jump-host:2200", "", "jump-host", 2200, false},
		{"host only", "gateway.local", "", "gateway.local", 22, false},
		{"bad port", "user@host:999999", "", "", 0, true},
		{"empty", "", "", "", 0, true},
		{"colon only", ":", "", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, host, port, err := ParseTunnelSpec(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr = %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if user != tt.wantUser || host != tt.wantHost || port != tt.wantPort {
				t.Errorf("got (%q, %q, %d), want (%q, %q, %d)",
					user, host, port, tt.wantUser, tt.wantHost, tt.wantPort)
			}
		})
	}
}

// ── ParsePortSpec ────────────────────────────────────────────────────

func TestParsePortSpec(t *testing.T) {
	tests := []struct {
		input     string
		wantStart int
		wantEnd   int
		wantErr   bool
	}{
		{"80", 80, 80, false},
		{"443", 443, 443, false},
		{"80-90", 80, 90, false},
		{"1-65535", 1, 65535, false},
		{"0", 0, 0, true},
		{"70000", 0, 0, true},
		{"abc", 0, 0, true},
		{"90-80", 0, 0, true},  // reversed range
		{"0-100", 0, 0, true},  // start below 1
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			pr, err := ParsePortSpec(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParsePortSpec(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if pr.Start != tt.wantStart || pr.End != tt.wantEnd {
				t.Errorf("got {%d, %d}, want {%d, %d}", pr.Start, pr.End, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

// ── PortRange.Expand ─────────────────────────────────────────────────

func TestPortRangeExpand(t *testing.T) {
	pr := PortRange{Start: 20, End: 25}
	got := pr.Expand()
	want := []int{20, 21, 22, 23, 24, 25}

	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d: got %d, want %d", i, got[i], want[i])
		}
	}
}

// ── Config.Validate ──────────────────────────────────────────────────

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "valid connect",
			cfg:     Config{Host: "example.com", Port: 80},
			wantErr: false,
		},
		{
			name:    "valid listen",
			cfg:     Config{Listen: true, LocalPort: 8080},
			wantErr: false,
		},
		{
			name:    "listen no port",
			cfg:     Config{Listen: true},
			wantErr: true,
		},
		{
			name:    "connect no host",
			cfg:     Config{Port: 80},
			wantErr: true,
		},
		{
			name:    "exec conflict",
			cfg:     Config{Host: "x", Port: 80, Execute: "a", Command: "b"},
			wantErr: true,
		},
		{
			name:    "udp tunnel",
			cfg:     Config{Host: "x", Port: 80, UDP: true, TunnelEnabled: true, TunnelHost: "gw", TunnelUser: "u"},
			wantErr: true,
		},
		{
			name:    "listen + scan",
			cfg:     Config{Listen: true, LocalPort: 80, ZeroIO: true},
			wantErr: true,
		},
		// ── reverse tunnel ─────────────────────────────────────
		{
			name:    "valid reverse tunnel",
			cfg:     Config{Listen: true, LocalPort: 8080, ReverseTunnelEnabled: true, ReverseTunnelHost: "gw", ReverseTunnelUser: "u", RemotePort: 9000},
			wantErr: false,
		},
		{
			name:    "reverse tunnel no listen",
			cfg:     Config{LocalPort: 8080, Host: "x", Port: 80, ReverseTunnelEnabled: true, ReverseTunnelHost: "gw", RemotePort: 9000},
			wantErr: true,
		},
		{
			name:    "reverse tunnel no remote port",
			cfg:     Config{Listen: true, LocalPort: 8080, ReverseTunnelEnabled: true, ReverseTunnelHost: "gw"},
			wantErr: true,
		},
		{
			name:    "reverse tunnel invalid remote port",
			cfg:     Config{Listen: true, LocalPort: 8080, ReverseTunnelEnabled: true, ReverseTunnelHost: "gw", RemotePort: 70000},
			wantErr: true,
		},
		{
			name:    "reverse tunnel + forward tunnel conflict",
			cfg:     Config{Listen: true, LocalPort: 8080, ReverseTunnelEnabled: true, ReverseTunnelHost: "gw", RemotePort: 9000, TunnelEnabled: true, TunnelHost: "gw2"},
			wantErr: true,
		},
		{
			name:    "reverse tunnel + UDP",
			cfg:     Config{Listen: true, LocalPort: 8080, ReverseTunnelEnabled: true, ReverseTunnelHost: "gw", RemotePort: 9000, UDP: true},
			wantErr: true,
		},
		{
			name:    "reverse tunnel no host",
			cfg:     Config{Listen: true, LocalPort: 8080, ReverseTunnelEnabled: true, RemotePort: 9000},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

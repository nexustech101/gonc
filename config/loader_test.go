package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadFromEnv_Host(t *testing.T) {
	t.Setenv("GONC_HOST", "test.example.com")
	cfg := &Config{}
	LoadFromEnv(cfg)
	if cfg.Host != "test.example.com" {
		t.Errorf("Host = %q, want %q", cfg.Host, "test.example.com")
	}
}

func TestLoadFromEnv_Port(t *testing.T) {
	t.Setenv("GONC_PORT", "8080")
	cfg := &Config{}
	LoadFromEnv(cfg)
	if cfg.LocalPort != 8080 {
		t.Errorf("LocalPort = %d, want 8080", cfg.LocalPort)
	}
}

func TestLoadFromEnv_Booleans(t *testing.T) {
	tests := []struct {
		key    string
		values []string
	}{
		{"GONC_LISTEN", []string{"1", "true", "yes", "TRUE", "Yes"}},
		{"GONC_UDP", []string{"1", "true"}},
		{"GONC_NO_DNS", []string{"true"}},
		{"GONC_KEEP_OPEN", []string{"1"}},
	}

	for _, tt := range tests {
		for _, v := range tt.values {
			t.Run(tt.key+"="+v, func(t *testing.T) {
				t.Setenv(tt.key, v)
				cfg := &Config{}
				LoadFromEnv(cfg)

				switch tt.key {
				case "GONC_LISTEN":
					if !cfg.Listen {
						t.Error("Listen should be true")
					}
				case "GONC_UDP":
					if !cfg.UDP {
						t.Error("UDP should be true")
					}
				case "GONC_NO_DNS":
					if !cfg.NoDNS {
						t.Error("NoDNS should be true")
					}
				case "GONC_KEEP_OPEN":
					if !cfg.KeepOpen {
						t.Error("KeepOpen should be true")
					}
				}
			})
		}
	}
}

func TestLoadFromEnv_Timeout(t *testing.T) {
	t.Setenv("GONC_TIMEOUT", "10")
	cfg := &Config{}
	LoadFromEnv(cfg)
	if cfg.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want 10s", cfg.Timeout)
	}
}

func TestLoadFromEnv_SSHFields(t *testing.T) {
	t.Setenv("GONC_TUNNEL", "admin@bastion:2222")
	t.Setenv("GONC_SSH_KEY", "/home/user/.ssh/id_rsa")
	t.Setenv("GONC_SSH_PASSWORD", "true")
	t.Setenv("GONC_SSH_AGENT", "1")
	t.Setenv("GONC_STRICT_HOSTKEY", "yes")
	t.Setenv("GONC_KNOWN_HOSTS", "/custom/known_hosts")

	cfg := &Config{}
	LoadFromEnv(cfg)

	if cfg.TunnelSpec != "admin@bastion:2222" {
		t.Errorf("TunnelSpec = %q", cfg.TunnelSpec)
	}
	if cfg.SSHKeyPath != "/home/user/.ssh/id_rsa" {
		t.Errorf("SSHKeyPath = %q", cfg.SSHKeyPath)
	}
	if !cfg.SSHPassword {
		t.Error("SSHPassword should be true")
	}
	if !cfg.UseSSHAgent {
		t.Error("UseSSHAgent should be true")
	}
	if !cfg.StrictHostKey {
		t.Error("StrictHostKey should be true")
	}
	if cfg.KnownHostsPath != "/custom/known_hosts" {
		t.Errorf("KnownHostsPath = %q", cfg.KnownHostsPath)
	}
}

func TestLoadFromEnv_ReverseTunnel(t *testing.T) {
	t.Setenv("GONC_REVERSE_TUNNEL", "serveo.net")
	t.Setenv("GONC_REMOTE_PORT", "80")
	t.Setenv("GONC_REMOTE_BIND_ADDRESS", "0.0.0.0")
	t.Setenv("GONC_KEEP_ALIVE", "60")
	t.Setenv("GONC_AUTO_RECONNECT", "true")

	cfg := &Config{}
	LoadFromEnv(cfg)

	if cfg.ReverseTunnelSpec != "serveo.net" {
		t.Errorf("ReverseTunnelSpec = %q", cfg.ReverseTunnelSpec)
	}
	if cfg.RemotePort != 80 {
		t.Errorf("RemotePort = %d", cfg.RemotePort)
	}
	if cfg.RemoteBindAddress != "0.0.0.0" {
		t.Errorf("RemoteBindAddress = %q", cfg.RemoteBindAddress)
	}
	if cfg.KeepAliveInterval != 60 {
		t.Errorf("KeepAliveInterval = %d", cfg.KeepAliveInterval)
	}
	if !cfg.AutoReconnect {
		t.Error("AutoReconnect should be true")
	}
}

func TestLoadFromEnv_NoOverrideWhenEmpty(t *testing.T) {
	// Ensure no GONC_ vars are set.
	os.Clearenv()

	cfg := &Config{Host: "original", LocalPort: 1234}
	LoadFromEnv(cfg)

	if cfg.Host != "original" {
		t.Errorf("Host was overridden: %q", cfg.Host)
	}
	if cfg.LocalPort != 1234 {
		t.Errorf("LocalPort was overridden: %d", cfg.LocalPort)
	}
}

func TestLoadFromEnv_InvalidIntIgnored(t *testing.T) {
	t.Setenv("GONC_PORT", "not-a-number")
	cfg := &Config{}
	LoadFromEnv(cfg)
	if cfg.LocalPort != 0 {
		t.Errorf("LocalPort should be 0 for invalid input, got %d", cfg.LocalPort)
	}
}

func TestLoadFromEnv_Verbose(t *testing.T) {
	t.Setenv("GONC_VERBOSE", "3")
	cfg := &Config{}
	LoadFromEnv(cfg)
	if cfg.Verbose != 3 {
		t.Errorf("Verbose = %d, want 3", cfg.Verbose)
	}
}

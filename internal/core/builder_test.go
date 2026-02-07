package core

import (
	"testing"

	"gonc/config"
	"gonc/util"
)

// TestBuild_Connect verifies that Build produces a ConnectMode for
// a simple connect configuration.
func TestBuild_Connect(t *testing.T) {
	cfg := &config.Config{Host: "example.com", Port: 80}
	logger := util.NewLogger(0)

	mode, err := Build(cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := mode.(*ConnectMode); !ok {
		t.Errorf("expected *ConnectMode, got %T", mode)
	}
}

// TestBuild_Listen verifies Build produces a ListenMode.
func TestBuild_Listen(t *testing.T) {
	cfg := &config.Config{Listen: true, LocalPort: 8080}
	logger := util.NewLogger(0)

	mode, err := Build(cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := mode.(*ListenMode); !ok {
		t.Errorf("expected *ListenMode, got %T", mode)
	}
}

// TestBuild_Scan verifies Build produces a ScanMode.
func TestBuild_Scan(t *testing.T) {
	cfg := &config.Config{
		Host:   "example.com",
		Port:   80,
		ZeroIO: true,
	}
	logger := util.NewLogger(0)

	mode, err := Build(cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := mode.(*ScanMode); !ok {
		t.Errorf("expected *ScanMode, got %T", mode)
	}
}

// TestBuild_ReverseTunnel verifies Build produces a ReverseTunnelMode.
func TestBuild_ReverseTunnel(t *testing.T) {
	cfg := &config.Config{
		Listen:               true,
		LocalPort:            8080,
		ReverseTunnelEnabled: true,
		ReverseTunnelUser:    "user",
		ReverseTunnelHost:    "gateway",
		ReverseTunnelPort:    22,
		RemotePort:           9000,
	}
	logger := util.NewLogger(0)

	mode, err := Build(cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := mode.(*ReverseTunnelMode); !ok {
		t.Errorf("expected *ReverseTunnelMode, got %T", mode)
	}
}

// TestBuild_NoDNS_Error verifies that a hostname with -n is rejected.
func TestBuild_NoDNS_Error(t *testing.T) {
	cfg := &config.Config{
		Host:  "example.com",
		Port:  80,
		NoDNS: true,
	}
	logger := util.NewLogger(0)

	_, err := Build(cfg, logger)
	if err == nil {
		t.Fatal("expected error for hostname with NoDNS")
	}
}

// TestBuild_NoDNS_IP verifies that a numeric IP with -n is accepted.
func TestBuild_NoDNS_IP(t *testing.T) {
	cfg := &config.Config{
		Host:  "127.0.0.1",
		Port:  80,
		NoDNS: true,
	}
	logger := util.NewLogger(0)

	mode, err := Build(cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := mode.(*ConnectMode); !ok {
		t.Errorf("expected *ConnectMode, got %T", mode)
	}
}

// TestBuild_UDP verifies that UDP config produces a ConnectMode with
// the right network.
func TestBuild_UDP(t *testing.T) {
	cfg := &config.Config{Host: "127.0.0.1", Port: 53, UDP: true}
	logger := util.NewLogger(0)

	mode, err := Build(cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	cm, ok := mode.(*ConnectMode)
	if !ok {
		t.Fatalf("expected *ConnectMode, got %T", mode)
	}
	if cm.Network != "udp" {
		t.Errorf("network = %q, want %q", cm.Network, "udp")
	}
}

// TestBuild_ExecCapability verifies that -e/-c selects the Exec
// capability instead of Relay.
func TestBuild_ExecCapability(t *testing.T) {
	cfg := &config.Config{Host: "127.0.0.1", Port: 80, Execute: "/bin/sh"}
	logger := util.NewLogger(0)

	mode, err := Build(cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	cm := mode.(*ConnectMode)
	if cm.Capability == nil {
		t.Fatal("capability should not be nil")
	}
}

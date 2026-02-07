package util

import (
	"testing"
)

func TestResolveAddr(t *testing.T) {
	tests := []struct {
		host    string
		port    int
		noDNS   bool
		want    string
		wantErr bool
	}{
		{"127.0.0.1", 80, true, "127.0.0.1:80", false},
		{"::1", 443, true, "[::1]:443", false},
		{"example.com", 80, false, "example.com:80", false},
		{"example.com", 80, true, "", true}, // hostname with noDNS
	}

	for _, tt := range tests {
		got, err := ResolveAddr(tt.host, tt.port, tt.noDNS)
		if (err != nil) != tt.wantErr {
			t.Errorf("ResolveAddr(%q,%d,%v) err=%v wantErr=%v",
				tt.host, tt.port, tt.noDNS, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("ResolveAddr(%q,%d,%v) = %q, want %q",
				tt.host, tt.port, tt.noDNS, got, tt.want)
		}
	}
}

func TestFormatAddr(t *testing.T) {
	if got := FormatAddr("1.2.3.4", 22); got != "1.2.3.4:22" {
		t.Errorf("got %q, want %q", got, "1.2.3.4:22")
	}
}

func TestFindFreePort(t *testing.T) {
	port, err := FindFreePort()
	if err != nil {
		t.Fatal(err)
	}
	if port < 1 || port > 65535 {
		t.Errorf("port %d out of range", port)
	}
}

func TestLookupHost_NoDNS(t *testing.T) {
	addrs, err := LookupHost("192.168.1.1", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(addrs) != 1 || addrs[0] != "192.168.1.1" {
		t.Errorf("got %v", addrs)
	}

	_, err = LookupHost("not-an-ip", true)
	if err == nil {
		t.Error("expected error for hostname with noDNS")
	}
}

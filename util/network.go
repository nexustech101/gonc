package util

import (
	"fmt"
	"net"
	"strconv"
)

// ResolveAddr builds a host:port string, validating that the host is a
// numeric IP when noDNS is true.
func ResolveAddr(host string, port int, noDNS bool) (string, error) {
	if noDNS {
		if net.ParseIP(host) == nil {
			return "", fmt.Errorf("cannot parse %q as an IP address (DNS disabled with -n)", host)
		}
	}
	return net.JoinHostPort(host, strconv.Itoa(port)), nil
}

// LookupHost resolves a hostname.  With noDNS it only accepts numeric IPs.
func LookupHost(host string, noDNS bool) ([]string, error) {
	if noDNS {
		if net.ParseIP(host) == nil {
			return nil, fmt.Errorf("cannot parse %q as an IP address (DNS disabled with -n)", host)
		}
		return []string{host}, nil
	}
	addrs, err := net.LookupHost(host)
	if err != nil {
		return nil, fmt.Errorf("DNS lookup for %q: %w", host, err)
	}
	return addrs, nil
}

// FormatAddr returns "host:port".
func FormatAddr(host string, port int) string {
	return net.JoinHostPort(host, strconv.Itoa(port))
}

// FindFreePort returns an available TCP port on 127.0.0.1.
func FindFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("finding free port: %w", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

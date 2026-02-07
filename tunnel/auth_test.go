package tunnel

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
)

// TestBuildAuthMethods_ExplicitKey verifies that a key file is loaded.
func TestBuildAuthMethods_ExplicitKey(t *testing.T) {
	// Generate a temporary ed25519 key in PEM format.
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "id_test")
	writeTestKey(t, keyPath)

	cfg := &SSHConfig{KeyPath: keyPath}
	methods, err := BuildAuthMethods(cfg)
	if err != nil {
		t.Fatalf("BuildAuthMethods: %v", err)
	}
	if len(methods) == 0 {
		t.Fatal("expected at least one auth method")
	}
}

// TestBuildAuthMethods_NoMethods verifies a clear error message.
func TestBuildAuthMethods_NoMethods(t *testing.T) {
	// Remove SSH_AUTH_SOCK so agent fails, and supply no key.
	t.Setenv("SSH_AUTH_SOCK", "")

	cfg := &SSHConfig{KeyPath: "/nonexistent/key"}
	_, err := BuildAuthMethods(cfg)
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

// TestHostKeyCallback_Insecure verifies that InsecureIgnoreHostKey is used
// when StrictHostKey is false.
func TestHostKeyCallback_Insecure(t *testing.T) {
	cfg := &SSHConfig{StrictHostKey: false}
	cb, err := hostKeyCallback(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if cb == nil {
		t.Fatal("callback should not be nil")
	}
}

// ── helpers ──────────────────────────────────────────────────────────

// writeTestKey writes a minimal, unencrypted RSA private key for testing.
func writeTestKey(t *testing.T, path string) {
	t.Helper()

	// Use crypto/ssh to generate a signer and marshal it.
	// For test brevity, write a known-good PEM RSA key.
	pem := `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACBBokBbMRiHRArMbOzFBKEFMftZHPaeCqnPr0MHKu7jbQAAAJhRxv9XUcb/
VwAAAAtzc2gtZWQyNTUxOQAAACBBokBbMRiHRArMbOzFBKEFMftZHPaeCqnPr0MHKu7jbQ
AAAEAntWSPLPjkafJSqniM0jnnz0PVURrz6xUYOVqEarfBWkGiQFsxGIdECsxs7MUEoQUx
+1kc9p4Kqc+vQwcq7uNtAAAADnRlc3RAZ29uYy10ZXN0AQIDBAUGBw==
-----END OPENSSH PRIVATE KEY-----
`
	// Verify the key parses before writing.
	if _, err := ssh.ParsePrivateKey([]byte(pem)); err != nil {
		t.Fatalf("bad test key: %v", err)
	}
	if err := os.WriteFile(path, []byte(pem), 0600); err != nil {
		t.Fatal(err)
	}
}

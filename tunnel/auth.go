package tunnel

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/term"
)

// BuildAuthMethods assembles an ordered list of SSH authentication
// methods from the tunnel configuration.
func BuildAuthMethods(cfg *SSHConfig) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	// 1. Explicit key file
	if cfg.KeyPath != "" {
		m, err := publicKeyAuth(cfg.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("key %s: %w", cfg.KeyPath, err)
		}
		methods = append(methods, m)
	}

	// 2. SSH agent (explicit flag)
	if cfg.UseAgent {
		m, err := agentAuth()
		if err != nil {
			return nil, fmt.Errorf("ssh-agent: %w", err)
		}
		methods = append(methods, m)
	}

	// 3. Interactive password
	if cfg.PromptPass {
		m, err := passwordAuth()
		if err != nil {
			return nil, err
		}
		methods = append(methods, m)
	}

	// 4. Fallback: try agent + common key files automatically.
	if len(methods) == 0 {
		methods = defaultAuthMethods()
	}

	if len(methods) == 0 {
		return nil, fmt.Errorf(
			"no SSH authentication methods available – " +
				"use --ssh-key, --ssh-password, or --ssh-agent")
	}
	return methods, nil
}

// ── individual auth builders ─────────────────────────────────────────

func publicKeyAuth(keyPath string) (ssh.AuthMethod, error) {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("reading key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(data)
	if err != nil {
		// If the key is encrypted, prompt for the passphrase.
		if _, ok := err.(*ssh.PassphraseMissingError); ok {
			fmt.Fprintf(os.Stderr, "Enter passphrase for %s: ", keyPath)
			pass, err2 := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(os.Stderr)
			if err2 != nil {
				return nil, fmt.Errorf("reading passphrase: %w", err2)
			}
			signer, err = ssh.ParsePrivateKeyWithPassphrase(data, pass)
			if err != nil {
				return nil, fmt.Errorf("decrypting key: %w", err)
			}
		} else {
			return nil, fmt.Errorf("parsing key: %w", err)
		}
	}
	return ssh.PublicKeys(signer), nil
}

func agentAuth() (ssh.AuthMethod, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, fmt.Errorf("SSH_AUTH_SOCK is not set")
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, fmt.Errorf("connecting to agent at %s: %w", sock, err)
	}
	return ssh.PublicKeysCallback(agent.NewClient(conn).Signers), nil
}

func passwordAuth() (ssh.AuthMethod, error) {
	fmt.Fprint(os.Stderr, "SSH password: ")
	pass, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("reading password: %w", err)
	}
	return ssh.Password(string(pass)), nil
}

// defaultAuthMethods tries the agent and the three most common key
// file names without any explicit user configuration.
func defaultAuthMethods() []ssh.AuthMethod {
	var out []ssh.AuthMethod

	// Agent
	if m, err := agentAuth(); err == nil {
		out = append(out, m)
	}

	// Common key names
	home, err := os.UserHomeDir()
	if err != nil {
		return out
	}
	for _, name := range []string{"id_ed25519", "id_rsa", "id_ecdsa"} {
		p := filepath.Join(home, ".ssh", name)
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if m, err := publicKeyAuth(p); err == nil {
			out = append(out, m)
		}
	}
	return out
}

// ── host-key verification ────────────────────────────────────────────

func hostKeyCallback(cfg *SSHConfig) (ssh.HostKeyCallback, error) {
	if !cfg.StrictHostKey {
		//nolint:gosec // user opted out of host key checking
		return ssh.InsecureIgnoreHostKey(), nil
	}

	khFile := cfg.KnownHosts
	if khFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("locating home directory: %w", err)
		}
		khFile = filepath.Join(home, ".ssh", "known_hosts")
	}

	cb, err := knownhosts.New(khFile)
	if err != nil {
		return nil, fmt.Errorf("loading known_hosts from %s: %w", khFile, err)
	}
	return cb, nil
}

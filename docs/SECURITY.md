# GoNC Security Guidelines

> Security considerations, hardening recommendations, and audit notes
> for the GoNC project.

---

## 1  Threat Model

GoNC is a network utility that:

- Opens TCP/UDP connections to arbitrary hosts and ports.
- Listens on local ports and accepts inbound connections.
- Executes external programs (`-e` / `-c` flags).
- Establishes SSH tunnels to remote gateways.
- Transfers data bidirectionally between endpoints.

**Trust boundary**: The user invoking `gonc` is trusted.  Remote peers
and SSH gateways are *not* trusted for data integrity.

---

## 2  SSH Tunnel Security

### 2.1  Host Key Verification

| Mode | Flag | Behaviour |
|------|------|-----------|
| **Relaxed** (default) | — | Accepts any host key (convenient but vulnerable to MITM) |
| **Strict** | `--strict-hostkey` | Verifies against `~/.ssh/known_hosts` or `--known-hosts` path |

**Recommendation**: Always use `--strict-hostkey` in production.

### 2.2  Authentication Methods

GoNC supports (in order of preference):

1. **SSH Agent** (`--ssh-agent`) — keys never touch disk.
2. **Private key file** (`--ssh-key`) — file permissions should be `0600`.
3. **Password prompt** (`--ssh-password`) — interactive only, never stored.
4. **Keyboard-interactive** — for services like serveo.net.

### 2.3  Key Material

- Private keys are loaded into memory only for the duration of the
  authentication handshake.
- Passwords are read from the terminal with echo disabled (`x/term`).
- No credentials are ever written to logs, even in debug mode.

---

## 3  Command Execution (`-e` / `-c`)

### 3.1  Risk

The `-e` and `-c` flags execute arbitrary programs with the same
privileges as the `gonc` process.  In listen mode, a remote peer can
send data that becomes `stdin` for the executed program.

### 3.2  Mitigations

- `-e` and `-c` are **mutually exclusive** (enforced by validation).
- No shell expansion is applied to `-e` paths — only the literal
  program name is executed.
- `-c` invokes the system shell (`/bin/sh -c` or `cmd /c`), so
  **the user is responsible for escaping**.

### 3.3  Recommendations

- Avoid `-e` / `-c` on publicly reachable listeners.
- Prefer `-c 'readonly-command'` over `-e /bin/sh`.
- Consider running `gonc` under a restricted user account.
- Use `--dry-run` to validate configuration before execution.

---

## 4  Network Security

### 4.1  Reverse Tunnel Exposure

When using `-R`, the remote gateway binds a port that forwards traffic
back through the SSH connection.  This means:

- The remote port is accessible to anyone who can reach the gateway.
- If `GatewayPorts` is enabled on the SSH server, the port may be
  reachable from the public internet.

**Mitigations**:
- Use `--remote-bind-address 127.0.0.1` to restrict to localhost.
- Use `--gateway-ports-check` to verify the server's configuration.
- Monitor active connections via the metrics collector.

### 4.2  UDP Mode

UDP (`-u`) has no built-in authentication or encryption.
Use SSH tunnels or VPNs for confidential UDP traffic.

### 4.3  Port Scanning

The `-z` (zero-I/O) mode performs TCP connect scans.  This is:
- Detectable by intrusion detection systems.
- Potentially illegal on networks you don't own.

Scan responsibly and only with authorization.

---

## 5  Input Validation

All user-supplied values are validated before use:

| Input | Validation |
|-------|-----------|
| Port numbers | 1–65535 range check |
| Tunnel specs | Regex match + range check |
| Timeouts | Non-negative, reasonable upper bound |
| SSH key paths | File existence check at load time |
| Exec paths | Program existence check (OS-level) |

Validation errors now use structured `ConfigError` types with
human-readable hints.

---

## 6  Dependency Security

### Current Dependencies

| Module | Version | Purpose | Risk |
|--------|---------|---------|------|
| `golang.org/x/crypto` | v0.24.0 | SSH client | Core crypto; track CVEs |
| `golang.org/x/term` | v0.23.0 | Password prompts | Minimal surface |
| `golang.org/x/sys` | v0.21.0 | Transitive (x/term) | OS interface |
| `github.com/spf13/pflag` | v1.0.5 | CLI flags | Stable, widely audited |

### Recommended Practices

1. **Pin dependencies** — use `go.sum` for integrity verification.
2. **Audit regularly** — run `govulncheck ./...` on each release.
3. **Update promptly** — especially `x/crypto` for SSH fixes.
4. **Minimal deps** — resist adding dependencies for features the
   stdlib can handle.

---

## 7  Build Security

### Reproducible Builds

```bash
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X gonc/cmd.version=$(git describe --tags)" -o gonc .
```

- `CGO_ENABLED=0` — static binary, no C dependencies.
- `-trimpath` — removes local filesystem paths from the binary.
- `-s -w` — strips symbol table and debug info.

### Docker

The multi-stage Dockerfile produces a `scratch`-based image with:
- No shell, no package manager, no attack surface.
- Non-root user (UID 65534).
- Read-only filesystem compatibility.

---

## 8  Reporting Vulnerabilities

If you discover a security issue, please:

1. **Do not** open a public GitHub issue.
2. Email the maintainers directly (see CODEOWNERS).
3. Allow 90 days for a fix before public disclosure.

---

## 9  Audit Checklist

Use this checklist before each release:

- [ ] `go vet ./...` passes cleanly
- [ ] `govulncheck ./...` reports no known vulnerabilities
- [ ] `go test -race ./...` passes (no data races)
- [ ] No credentials in logs (grep for password, key, secret, token)
- [ ] SSH host key verification documented in release notes
- [ ] Docker image rebuilt from clean base
- [ ] CHANGELOG updated with security-relevant changes

# GoNC – Network Connectivity Tool



A cross-platform **netcat** clone written in Go with native **SSH tunneling** support.

GoNC provides all standard netcat functionality – TCP/UDP client and server,
port scanning, command execution – plus the ability to route any connection
through an encrypted SSH tunnel without external tools.

---

## Quick Start

```bash
# Connect to a remote port
gonc example.com 80

# Listen for inbound connections
gonc -l -p 8080

# Scan ports
gonc -vz host.example.com 20-25 80 443

# Connect through an SSH tunnel
gonc -T admin@bastion.example.com internal-db 5432
```

## Installation

### From Source

```bash
git clone <repo-url> && cd gonc
go build -o gonc .            # Linux / macOS
go build -o gonc.exe .        # Windows
```

### Cross-compile

```bash
make build-all   # produces Linux, macOS, and Windows binaries
```

### Using `go install`

```bash
go install gonc@latest
```

## Features

| Feature | Flag | Description |
|---------|------|-------------|
| TCP connect | `gonc host port` | Standard client mode |
| TCP listen | `-l -p PORT` | Accept inbound connections |
| UDP mode | `-u` | Datagram transport |
| Port scan | `-z` | Zero-I/O scan |
| Keep open | `-k` | Accept multiple connections |
| Timeout | `-w SECS` | Connection / idle timeout |
| Exec | `-e PROG` | Bind a program to the socket |
| Shell cmd | `-c CMD` | Bind a shell command |
| SSH tunnel | `-T user@host` | Route through SSH gateway |
| Reverse tunnel | `-R host` | Expose local service on remote gateway |
| Remote port | `--remote-port PORT` | Port to bind on remote gateway (for -R) |
| Remote bind | `--remote-bind-address` | Remote bind address (default: server decides) |
| GatewayPorts check | `--gateway-ports-check` | Validate GatewayPorts before tunneling |
| Keep-alive | `--keep-alive SECS` | SSH keepalive interval (default 30) |
| Auto-reconnect | `--auto-reconnect` | Reconnect on tunnel drop |
| SSH key | `--ssh-key PATH` | Private key authentication |
| SSH password | `--ssh-password` | Interactive password prompt |
| SSH agent | `--ssh-agent` | Use running SSH agent |
| Host keys | `--strict-hostkey` | Verify server fingerprints |
| Verbose | `-v` / `-vv` | Increase output detail |
| No DNS | `-n` | Numeric-only, skip DNS |

## SSH Tunnel Usage

GoNC can wrap any TCP connection in an SSH tunnel, allowing you to reach
hosts that are only accessible from a bastion / jump server:

```
local machine ──SSH──▶ gateway ──TCP──▶ destination:port
```

### Examples

```bash
# Basic tunnel
gonc -T user@bastion.example.com internal-service 8080

# With explicit key
gonc -T deploy@gateway --ssh-key ~/.ssh/deploy_key db-server 5432

# Password authentication
gonc -T admin@jump-host --ssh-password target 22

# Scan through tunnel
gonc -vz -T user@bastion 10.0.0.5 22 80 443 3306

# Pipe data through tunnel
echo "SELECT 1" | gonc -T dba@bastion mysql-internal 3306
```

### Authentication Order

When no explicit auth flags are given, GoNC tries:

1. SSH agent (via `SSH_AUTH_SOCK` on Unix, or the OpenSSH named pipe on Windows)
2. `~/.ssh/id_ed25519`
3. `~/.ssh/id_rsa`
4. `~/.ssh/id_ecdsa`
5. Keyboard-interactive (auto-enabled for reverse tunnels — needed by
   serveo.net and localhost.run)

## Reverse SSH Tunnel (Expose Local Service)

GoNC can expose a local TCP service on a remote SSH gateway using reverse
tunneling, equivalent to `ssh -R`.  Remote clients connecting to the
gateway are transparently forwarded back to your local machine:

```
Remote Client ──▶ SSH Gateway (port 9000) ──SSH──▶ GoNC ──TCP──▶ Local Service (port 8080)
```

The `-R` flag **implies listen mode** (`-l`), so you only need to specify
your local port with `-p` and the remote port with `--remote-port`.

### Examples

```bash
# Expose local port 8080 on gateway port 9000
gonc -p 8080 -R user@gateway.example.com --remote-port 9000

# With a specific remote bind address
gonc -p 3306 -R admin@bastion --remote-port 3306 --remote-bind-address 10.0.1.5

# Validate GatewayPorts before connecting
gonc -p 8080 -R user@gateway --remote-port 9000 --gateway-ports-check

# Auto-reconnect on tunnel drop
gonc -p 8080 -R user@gateway --remote-port 9000 --auto-reconnect

# Custom keepalive interval
gonc -p 8080 -R user@gateway --remote-port 9000 --keep-alive 15
```

### Requirements

- The remote SSH server must have `GatewayPorts yes` or `GatewayPorts clientspecified` in `sshd_config` for external access (not needed for public tunnel services).
- Binding ports below 1024 on the remote requires root privileges.
- Use `--gateway-ports-check` to validate before establishing the tunnel.

### Use Cases

- **Expose local dev server** to a remote team
- **Webhook testing** with external services (GitHub, Stripe, etc.)
- **Bypass NAT/firewall** for incoming connections
- **Share localhost** securely through services like serveo.net

---

## Exposing Localhost to the Internet (Developer Tunnels)

One of the most useful features of GoNC's reverse tunnel is the ability to
expose a local development server to the public internet using free SSH
tunnel services.  This is ideal for:

- Sharing a local website or API with a colleague
- Testing webhooks from GitHub, Stripe, Slack, etc.
- Demoing a project without deploying to a server
- Testing mobile apps against a local backend

### Serveo.net

[Serveo](https://serveo.net) is a free SSH tunnel service — no account or
client installation required.  GoNC works with it out of the box:

```bash
# Start your local dev server (e.g., on port 3000)
npm run dev   # or python -m http.server 3000, etc.

# In another terminal, create the tunnel
gonc -p 3000 -R serveo.net --remote-port 80
```

GoNC will print the generated public URL:

```
reverse tunnel established: :80 (remote) → 127.0.0.1:3000 (local)
Forwarding HTTP traffic from https://abc123-71-60-35-103.serveousercontent.com
```

Open the printed URL in a browser to access your local server.

**How it works:** GoNC opens an SSH connection to serveo.net, authenticates
via keyboard-interactive (no keys or password needed), and requests a remote
port forward.  Serveo assigns a unique subdomain and routes HTTP traffic
from that subdomain back through the SSH tunnel to your local port.

**Tips:**

- No `-l` flag needed — `-R` implies listen mode automatically.
- No `user@` prefix needed — GoNC defaults to your OS username, and
  serveo.net ignores the username anyway.
- The `--remote-port 80` tells serveo to expose HTTP traffic; for
  HTTPS the URL is automatically generated.
- Add `-v` for connection details or `-vv` for full debug output.
- Use `--auto-reconnect` for long-running sessions that should survive
  network blips.
- Use `--keep-alive 15` for aggressive keepalive on unreliable connections.

```bash
# Verbose mode to see connection details
gonc -v -p 3000 -R serveo.net --remote-port 80

# Auto-reconnect for long-running tunnels
gonc -p 3000 -R serveo.net --remote-port 80 --auto-reconnect --keep-alive 15

# Expose port 8080 instead
gonc -p 8080 -R serveo.net --remote-port 80
```

### localhost.run

[localhost.run](https://localhost.run) is another free tunnel service that
works over SSH.  It functions similarly to serveo.net:

```bash
gonc -p 3000 -R localhost.run --remote-port 80
```

### Cloudflare Tunnels

[Cloudflare Tunnels](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/)
(formerly Argo Tunnel) use the `cloudflared` daemon rather than SSH, so
they are **not directly compatible** with GoNC's `-R` flag.  However, if
you have an SSH-accessible server with Cloudflare DNS, you can achieve a
similar result:

```bash
# 1. Set up a VPS / jump server with sshd + GatewayPorts enabled
# 2. Point a Cloudflare DNS record to the VPS (e.g., demo.example.com → VPS IP)
# 3. Use GoNC to expose your local server on the VPS
gonc -p 3000 -R user@your-vps.example.com --remote-port 8080 --auto-reconnect

# Now demo.example.com:8080 routes through the VPS back to your localhost:3000.
```

For a fully managed Cloudflare Tunnel with HTTPS and custom domains, use
`cloudflared tunnel` — it handles certificate provisioning and DNS
automatically, but requires installing their CLI.

### Comparison of Tunnel Services

| Service | Auth | Custom domain | HTTPS | Setup |
|---------|------|--------------|-------|-------|
| **serveo.net** | None (auto) | Paid | ✅ Auto | `gonc -p 3000 -R serveo.net --remote-port 80` |
| **localhost.run** | None (auto) | No | ✅ Auto | `gonc -p 3000 -R localhost.run --remote-port 80` |
| **Own SSH server** | Key/password | You control | You configure | `gonc -p 3000 -R user@server --remote-port 80` |
| **Cloudflare Tunnel** | OAuth/cert | ✅ | ✅ | Requires `cloudflared` (not SSH) |

## Build

### Prerequisites

* Go 1.22+

### Commands

```bash
make build         # build for current OS
make build-all     # cross-compile all targets
make test          # run tests with race detector
make lint          # golangci-lint
make clean         # remove artefacts
```

### Windows Version Info

The optional `resource/resource.json` can be used with
[goversioninfo](https://github.com/josephspurrier/goversioninfo) to embed
PE metadata (FileDescription, CompanyName, etc.) into the Windows binary.

```bash
go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest
goversioninfo -o resource.syso resource/resource.json
go build -o gonc.exe .
```

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for design details.

## Project Layout

```
gonc/
├── main.go                    Entry point
├── go.mod / go.sum            Module dependencies
├── Makefile                   Build, test, lint, clean targets
├── cmd/
│   └── root.go                CLI flag parsing (pflag)
├── config/
│   ├── config.go              Configuration & validation
│   └── config_test.go
├── netcat/
│   ├── netcat.go              Run dispatcher
│   ├── client.go              TCP/UDP client mode
│   ├── client_test.go
│   ├── server.go              Listen mode
│   ├── server_test.go
│   ├── reverse.go             Reverse tunnel dispatch (→ tunnel pkg)
│   ├── transfer.go            Exec / command binding
│   ├── scanner.go             Port scanning (-z)
│   └── scanner_test.go
├── tunnel/
│   ├── tunnel.go              Tunnel interface
│   ├── ssh.go                 SSH forward tunnel + SSHConfig struct
│   ├── auth.go                Auth methods (keys, agent, keyboard-interactive)
│   ├── auth_test.go
│   ├── reverse.go             Reverse SSH tunnel engine (ssh -R)
│   ├── reverse_test.go        + custom forwarded-tcpip channel handler
│   └── manager.go             Lifecycle / health
├── util/
│   ├── io.go                  Bidirectional copy
│   ├── io_test.go
│   ├── network.go             Address helpers
│   ├── network_test.go
│   └── logger.go              Levelled logging
├── resource/
│   └── resource.json          Windows PE version metadata
├── scripts/
│   ├── build-all.sh           Cross-compile helper
│   └── integration-test.sh    Docker E2E test runner
├── Dockerfile                 Multi-stage (deps → test → builder → final → dist)
├── docker-compose.yaml        Dev/test orchestration (build, test, integration)
├── docker-compose.prod.yaml   Hardened production deployment
├── .dockerignore
├── README-DOCKER.md           Docker usage guide
├── ARCHITECTURE.md            Design documentation
└── EXAMPLES.md                Extended usage examples
```

## License

MIT – see individual source files.

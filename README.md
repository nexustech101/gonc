# 🌐 GoNC — Cross-Platform Netcat with SSH Tunneling

![Go](https://img.shields.io/badge/go-1.22+-00ADD8.svg?logo=go&logoColor=white)
![SSH](https://img.shields.io/badge/ssh-tunneling-black.svg?logo=openssh&logoColor=white)
![Docker](https://img.shields.io/badge/docker-compose-2496ED.svg?logo=docker&logoColor=white)
![Platforms](https://img.shields.io/badge/platforms-linux%20%7C%20macOS%20%7C%20windows-lightgrey.svg)
![License](https://img.shields.io/badge/license-MIT-green.svg)
![Release](https://img.shields.io/badge/release-v1.2.0-blue.svg)

A production-ready **netcat** reimplementation in Go with first-class **SSH tunnel** support. GoNC provides all classic netcat functionality — TCP/UDP client & server, port scanning, command execution — plus the ability to route any connection through an encrypted SSH tunnel or expose local services to the internet via reverse tunneling. Single static binary, zero configuration files, cross-platform.

---

## 📑 Table of Contents

- [Quick Start](#-quick-start)
- [Installation](#-installation)
- [Features](#-features)
- [SSH Forward Tunnel](#-ssh-forward-tunnel)
- [Reverse SSH Tunnel](#-reverse-ssh-tunnel--expose-local-services)
- [Developer Tunnels (Expose Localhost)](#-developer-tunnels--expose-localhost-to-the-internet)
- [Environment Variables](#-environment-variables)
- [Docker](#-docker)
- [Build](#-build)
- [Architecture](#-architecture)
- [Project Layout](#-project-layout)
- [Security](#-security)
- [License](#-license)

---

## 🚀 Quick Start

```bash
# Connect to a remote port
gonc example.com 80

# Listen for inbound connections
gonc -l -p 8080

# Scan a range of ports
gonc -vz host.example.com 20-25 80 443

# Connect through an SSH tunnel
gonc -T admin@bastion.example.com internal-db 5432

# Expose local port 3000 on the internet (via serveo.net)
gonc -p 3000 -R serveo.net --remote-port 80
```

---

## 📦 Installation

### From Source

```bash
git clone https://github.com/nexustech101/gonc.git && cd gonc
go build -o gonc .            # Linux / macOS
go build -o gonc.exe .        # Windows
```

### Cross-Compile All Platforms

```bash
make build-all   # linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
```

### Using `go install`

```bash
go install gonc@latest
```

### Docker

```bash
docker compose --profile run build
docker compose run gonc example.com 80
```

---

## ✨ Features

| Feature | Flag | Description |
|:--------|:-----|:------------|
| **TCP connect** | `gonc host port` | Standard client mode |
| **TCP listen** | `-l -p PORT` | Accept inbound connections |
| **UDP mode** | `-u` | Datagram transport |
| **Port scan** | `-z` | Zero-I/O scan with concurrency |
| **Keep open** | `-k` | Accept multiple connections |
| **Timeout** | `-w SECS` | Connection / idle timeout |
| **Exec** | `-e PROG` | Bind a program to the socket |
| **Shell cmd** | `-c CMD` | Bind a shell command |
| **Verbose** | `-v` / `-vv` | Increase output detail |
| **No DNS** | `-n` | Numeric-only, skip DNS resolution |
| **Dry run** | `--dry-run` | Validate config without executing |

### 🔐 SSH Tunnel Features

| Feature | Flag | Description |
|:--------|:-----|:------------|
| **Forward tunnel** | `-T user@host` | Route through SSH gateway |
| **Reverse tunnel** | `-R host` | Expose local service on remote gateway |
| **Remote port** | `--remote-port PORT` | Port to bind on remote side |
| **Remote bind** | `--remote-bind-address` | Remote bind address |
| **GatewayPorts check** | `--gateway-ports-check` | Validate server config before tunneling |
| **Keep-alive** | `--keep-alive SECS` | SSH keepalive interval (default 30) |
| **Auto-reconnect** | `--auto-reconnect` | Reconnect on tunnel drop |
| **SSH key** | `--ssh-key PATH` | Private key authentication |
| **SSH password** | `--ssh-password` | Interactive password prompt |
| **SSH agent** | `--ssh-agent` | Use running SSH agent |
| **Host key verify** | `--strict-hostkey` | Verify server fingerprints |

---

## 🔒 SSH Forward Tunnel

Route any TCP connection through an encrypted SSH tunnel to reach hosts behind a bastion / jump server:

```
┌──────────┐        SSH        ┌──────────┐        TCP        ┌─────────────┐
│  Local    │ ═══════════════▶ │ Gateway  │ ──────────────── ▶│ Destination │
│  Machine  │   (encrypted)    │ (bastion)│                   │  host:port  │
└──────────┘                   └──────────┘                   └─────────────┘
```

### Examples

```bash
# Basic tunnel through bastion
gonc -T user@bastion.example.com internal-service 8080

# With explicit SSH key
gonc -T deploy@gateway --ssh-key ~/.ssh/deploy_key db-server 5432

# Password authentication
gonc -T admin@jump-host --ssh-password target 22

# Port scan through tunnel
gonc -vz -T user@bastion 10.0.0.5 22 80 443 3306

# Pipe data through tunnel
echo "SELECT 1" | gonc -T dba@bastion mysql-internal 3306
```

### 🔑 Authentication Order

When no explicit auth flags are given, GoNC tries in order:

| Priority | Method | Details |
|:--------:|:-------|:--------|
| 1 | **SSH Agent** | `SSH_AUTH_SOCK` (Unix) or OpenSSH named pipe (Windows) |
| 2 | **Ed25519 key** | `~/.ssh/id_ed25519` |
| 3 | **RSA key** | `~/.ssh/id_rsa` |
| 4 | **ECDSA key** | `~/.ssh/id_ecdsa` |
| 5 | **Keyboard-interactive** | Auto-enabled for reverse tunnels (serveo.net, localhost.run) |

---

## 🔄 Reverse SSH Tunnel — Expose Local Services

Expose a local TCP service on a remote SSH gateway, equivalent to `ssh -R`. Remote clients connecting to the gateway are transparently forwarded back to your local machine:

```
┌──────────────┐              ┌──────────────┐    SSH tunnel    ┌──────────┐        TCP        ┌───────────────┐
│ Remote Client│ ──────────▶  │ SSH Gateway  │ ═══════════════▶ │  GoNC    │ ──────────────── ▶│ Local Service │
│              │              │  (port 9000) │   (encrypted)    │          │                   │  (port 8080)  │
└──────────────┘              └──────────────┘                  └──────────┘                   └───────────────┘
```

> **Note:** The `-R` flag implies listen mode (`-l`), so you only need `-p` for the local port and `--remote-port` for the remote side.

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

| Requirement | Details |
|:------------|:--------|
| **GatewayPorts** | SSH server must have `GatewayPorts yes` or `clientspecified` in `sshd_config` for external access |
| **Privileged ports** | Binding ports < 1024 on remote requires root |
| **Validation** | Use `--gateway-ports-check` to verify before tunneling |

### Use Cases

- 🌍 **Expose local dev server** to a remote team
- 🔗 **Webhook testing** with external services (GitHub, Stripe, etc.)
- 🛡️ **Bypass NAT/firewall** for incoming connections
- 🤝 **Share localhost** securely through tunnel services

---

## 🌍 Developer Tunnels — Expose Localhost to the Internet

One of GoNC's most powerful features is the ability to expose a local dev server to the public internet using free SSH tunnel services. Perfect for:

- 🔗 Sharing a local website or API with a colleague
- 🪝 Testing webhooks from GitHub, Stripe, Slack, etc.
- 🖥️ Demoing a project without deploying to a server
- 📱 Testing mobile apps against a local backend

### Serveo.net

[Serveo](https://serveo.net) is a free SSH tunnel service — **no account or installation required**. GoNC works with it out of the box:

```bash
# Start your local dev server (e.g., on port 3000)
npm run dev   # or python -m http.server 3000

# In another terminal, create the tunnel
gonc -p 3000 -R serveo.net --remote-port 80
```

GoNC will print the generated public URL:

```
reverse tunnel established: :80 (remote) → 127.0.0.1:3000 (local)
Forwarding HTTP traffic from https://abc123.serveousercontent.com
```

> **💡 Tips:**
> - No `-l` flag needed — `-R` implies listen mode automatically
> - No `user@` prefix needed — serveo.net ignores the username
> - Add `-v` for details or `-vv` for full debug output
> - Use `--auto-reconnect` for long-running sessions
> - Use `--keep-alive 15` on unreliable connections

### localhost.run

[localhost.run](https://localhost.run) is another free tunnel service that works identically:

```bash
gonc -p 3000 -R localhost.run --remote-port 80
```

### Cloudflare Tunnels

[Cloudflare Tunnels](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/) use the `cloudflared` daemon (not SSH), so they are **not directly compatible** with GoNC's `-R` flag. However, you can achieve a similar result via your own VPS:

```bash
# 1. Set up a VPS with sshd + GatewayPorts enabled
# 2. Point a Cloudflare DNS record to the VPS IP
# 3. Use GoNC to expose your local server on the VPS
gonc -p 3000 -R user@your-vps.example.com --remote-port 8080 --auto-reconnect
```

### 📊 Tunnel Service Comparison

| Service | Auth | Custom Domain | HTTPS | Command |
|:--------|:-----|:-------------|:------|:--------|
| **serveo.net** | None (auto) | Paid | ✅ Auto | `gonc -p 3000 -R serveo.net --remote-port 80` |
| **localhost.run** | None (auto) | No | ✅ Auto | `gonc -p 3000 -R localhost.run --remote-port 80` |
| **Own SSH server** | Key / password | You control | You configure | `gonc -p 3000 -R user@server --remote-port 80` |
| **Cloudflare Tunnel** | OAuth / cert | ✅ | ✅ | Requires `cloudflared` (not SSH) |

---

## ⚙️ Environment Variables

GoNC supports configuration via environment variables with the `GONC_` prefix. **Precedence: CLI flags > Environment > Defaults.**

| Variable | Description |
|:---------|:------------|
| `GONC_HOST` | Default target host |
| `GONC_PORT` | Default target port |
| `GONC_LISTEN` | Enable listen mode |
| `GONC_UDP` | Enable UDP mode |
| `GONC_VERBOSE` | Verbosity level |
| `GONC_TUNNEL` | SSH tunnel spec (`user@host:port`) |
| `GONC_SSH_KEY` | SSH private key path |
| `GONC_SSH_AGENT` | Use SSH agent |
| `GONC_STRICT_HOSTKEY` | Enable strict host key verification |
| `GONC_REVERSE_TUNNEL` | Reverse tunnel spec |
| `GONC_REMOTE_PORT` | Remote port for reverse tunnel |
| `GONC_AUTO_RECONNECT` | Auto-reconnect on tunnel drop |
| `GONC_SSH_PASSWORD_VALUE` | SSH password for non-interactive / CI use |

---

## 🐳 Docker

GoNC ships with a multi-stage Dockerfile and full Docker Compose orchestration.

### Quick Commands

```bash
# Run unit tests with race detector + coverage
docker compose --profile test up

# Cross-compile all platform binaries → ./dist/
docker compose --profile build up

# Run the full integration suite (direct TCP, port scan, SSH tunnel)
docker compose --profile integration up --abort-on-container-exit

# Run gonc in a container
docker compose --profile run build
docker compose run gonc -vz example.com 80
```

### Docker Stages

| Stage | Purpose |
|:------|:--------|
| `deps` | Download and cache Go modules |
| `test` | `go vet` + `go test -race` with coverage |
| `builder` | Cross-compile static binaries (linux, macOS, windows) |
| `final` | Minimal Alpine runtime image (~10 MB) |
| `dist` | Extract all binaries from scratch image |

> 📖 See [README-DOCKER.md](README-DOCKER.md) for the full Docker usage guide.

---

## 🔨 Build

### Prerequisites

- **Go 1.22+**

### Makefile Targets

```bash
make build         # build for current OS (version auto-detected from git)
make build-all     # cross-compile all targets
make test          # go test -race -count=1 -timeout 60s ./...
make bench         # run benchmarks
make coverage      # generate HTML coverage report
make check         # go vet + go test (CI gate)
make clean         # remove build artefacts
```

### Windows Version Info (Optional)

Embed PE metadata (FileDescription, CompanyName, etc.) into the Windows binary:

```bash
go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest
goversioninfo -o resource.syso resource/resource.json
go build -o gonc.exe .
```

---

## 🏗️ Architecture

GoNC follows a clean layered architecture with well-separated concerns:

```
┌────────────┐
│   cmd/     │  CLI flag parsing (pflag)
├────────────┤
│  config/   │  Configuration, validation, env loading
├────────────┤
│  netcat/   │  Core logic: client, server, scanner, reverse dispatch
├────────────┤
│  tunnel/   │  SSH forward + reverse tunneling engine
├────────────┤
│  internal/ │  errors · retry (backoff + circuit breaker) · metrics
├────────────┤
│   util/    │  Logger, I/O helpers, network utils, buffer pool
└────────────┘
```

> 📖 See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed design documentation.
>
> 🔒 See [docs/SECURITY.md](docs/SECURITY.md) for threat model and hardening guide.
>
> 📋 See [docs/REFACTORING.md](docs/REFACTORING.md) for the full refactoring changelog.

---

## 📁 Project Layout

```
gonc/
├── main.go                         Entry point
├── go.mod / go.sum                 Module dependencies
├── Makefile                        Build, test, bench, coverage, check
│
├── cmd/
│   ├── root.go                     CLI flags → Config → core.Build() → Run
│   └── root_test.go                CLI integration tests
│
├── config/
│   ├── config.go                   Config struct & validation
│   ├── config_test.go              Validation tests
│   ├── defaults.go                 Centralized default constants
│   └── loader.go                   Environment variable loader
│
├── internal/
│   ├── core/                       Orchestration layer
│   │   ├── mode.go                 Mode interface
│   │   ├── builder.go              Build(cfg) → Mode (single dispatch point)
│   │   ├── connect.go              ConnectMode: Dialer + Capability
│   │   ├── listen.go               ListenMode: accept → Capability per conn
│   │   ├── scan.go                 ScanMode: concurrent port probing
│   │   └── reverse.go              ReverseTunnelMode
│   ├── transport/                  How data moves
│   │   ├── transport.go            Dialer interface
│   │   ├── tcp.go                  TCPDialer (plain TCP)
│   │   ├── udp.go                  UDPDialer (plain UDP)
│   │   └── ssh.go                  SSHDialer (lazy SSH tunnel wrapper)
│   ├── capability/                 What happens over a connection
│   │   ├── capability.go           Capability interface
│   │   ├── relay.go                Relay: stdin/stdout ↔ connection
│   │   └── exec.go                 Exec: wire conn to child process
│   ├── session/                    Connection lifecycle
│   │   └── session.go              Session: Conn + I/O + Logger
│   ├── errors/                     Domain error types
│   ├── retry/                      Exponential backoff + circuit breaker
│   └── metrics/                    Lock-free atomic counters
│
├── tunnel/
│   ├── tunnel.go                   Tunnel interface
│   ├── ssh.go                      SSH forward tunnel + config
│   ├── auth.go / auth_test.go      Auth methods (key, agent, password, KI)
│   ├── reverse_tunnel.go           Reverse tunnel lifecycle
│   ├── reverse_forwarder.go        Connection bridging + metrics
│   ├── reverse_health.go           Keepalive & reconnection
│   ├── reverse_dial.go             SSH dial + GatewayPorts validation
│   ├── reverse_listener.go         Custom forwarded-tcpip handler
│   └── manager.go                  Lifecycle management
│
├── util/
│   ├── io.go / io_test.go          Bidirectional copy
│   ├── network.go / network_test.go Address helpers
│   ├── logger.go                   Levelled logger with timestamps
│   └── pool.go                     sync.Pool buffer reuse
│
├── docs/
│   ├── REFACTORING.md              Refactoring changelog
│   └── SECURITY.md                 Threat model & hardening guide
│
├── scripts/
│   ├── build-all.sh                Cross-compile helper
│   ├── integration-test.sh         Docker E2E test runner
│   └── enable-forwarding.sh        Test SSH forwarding config
│
├── resource/
│   └── resource.json               Windows PE version metadata
│
├── Dockerfile                      Multi-stage (deps → test → builder → final → dist)
├── docker-compose.yaml             Dev/test orchestration
├── docker-compose.prod.yaml        Hardened production deployment
├── .dockerignore
├── ARCHITECTURE.md                 Design documentation
├── EXAMPLES.md                     Extended usage examples
└── README-DOCKER.md                Docker usage guide
```

---

## 🔒 Security

GoNC follows a clear trust model: **the invoking user is trusted; remote peers and SSH gateways are not.**

Key security features:

- 🔑 **SSH host key verification** via `--strict-hostkey` (off by default for convenience)
- 🔐 **Multiple auth methods** — key files, SSH agent, password prompt, keyboard-interactive
- 🛡️ **No plaintext secrets** — passwords read from terminal or `GONC_SSH_PASSWORD_VALUE` env var
- ⚠️ **Exec/command flags** (`-e` / `-c`) require explicit opt-in
- 📦 **Static binary** — no dynamic dependencies, minimal attack surface
- 🔒 **Build security** — `CGO_ENABLED=0`, `-trimpath`, stripped symbols

> 📖 See [docs/SECURITY.md](docs/SECURITY.md) for the full threat model and hardening guide.

---

## 📄 License

MIT — see individual source files for details.

---

<p align="center">
  <b>GoNC</b> — netcat, reimagined for the tunnel age. 🌐
</p>

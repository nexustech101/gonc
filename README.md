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

1. SSH agent (via `SSH_AUTH_SOCK`)
2. `~/.ssh/id_ed25519`
3. `~/.ssh/id_rsa`
4. `~/.ssh/id_ecdsa`

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
├── main.go              Entry point
├── cmd/root.go          CLI flag parsing
├── config/config.go     Configuration & validation
├── netcat/
│   ├── netcat.go        Run dispatcher
│   ├── client.go        TCP/UDP client mode
│   ├── server.go        Listen mode
│   ├── transfer.go      Exec / command binding
│   └── scanner.go       Port scanning (-z)
├── tunnel/
│   ├── tunnel.go        Tunnel interface
│   ├── ssh.go           SSH implementation
│   ├── auth.go          Auth method builders
│   └── manager.go       Lifecycle / health
└── util/
    ├── io.go            Bidirectional copy
    ├── network.go       Address helpers
    └── logger.go        Levelled logging
```

## License

MIT – see individual source files.

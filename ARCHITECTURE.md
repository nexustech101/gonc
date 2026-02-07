# GoNC – Architecture

## Design Goals

1. **Single static binary** – zero runtime dependencies.
2. **Cross-platform** – identical behaviour on Linux, macOS, and Windows.
3. **Minimal dependency surface** – only `x/crypto`, `x/term`, and `pflag`.
4. **Clean separation of concerns** – each package has a single responsibility.
5. **Testability** – every subsystem is unit-testable in isolation.

## Package Overview

```
main.go
  ↓
cmd/root.go          Parse flags → build Config → build Tunnel → build NetCat → Run
  ↓                                    ↓                  ↓
config/config.go     Immutable config struct + validation
  ↓
netcat/netcat.go     Orchestrator: dispatches to client / server / scanner
  ├─ client.go       TCP & UDP connect mode
  ├─ server.go       TCP & UDP listen mode (with keep-open)
  ├─ transfer.go     Exec / command binding via os/exec
  └─ scanner.go      Concurrent port scanning
  ↓
tunnel/tunnel.go     Interface definition
  ├─ ssh.go          SSH implementation (x/crypto/ssh)
  ├─ auth.go         Authentication method builders
  └─ manager.go      Health monitoring goroutine
  ↓
util/
  ├─ io.go           BidirectionalCopy (goroutine-safe, context-aware)
  ├─ network.go      Address formatting, DNS, free-port finder
  └─ logger.go       Levelled stderr logger
```

## Data Flow

### Direct Connection

```
stdin ──▶ ┌──────────┐        ┌─────────────┐
          │  NetCat   │──TCP──▶│  Remote Host │
stdout ◀──┤  client   │◀──────┤             │
          └──────────┘        └─────────────┘
```

### SSH Tunnel Connection

```
stdin ──▶ ┌──────────┐        ┌────────────┐        ┌─────────────┐
          │  NetCat   │──SSH──▶│  Gateway    │──TCP──▶│  Destination│
stdout ◀──┤  client   │◀──────┤  (bastion)  │◀──────┤             │
          └──────────┘        └────────────┘        └─────────────┘
```

The tunnel is transparent to the netcat core:

```go
// Without tunnel
conn, _ = net.Dial("tcp", addr)

// With tunnel – identical interface
conn, _ = sshTunnel.Dial(ctx, "tcp", addr)
```

`ssh.Client.Dial()` returns a standard `net.Conn`, so the rest of the
code does not need to know whether a tunnel is involved.

## Concurrency Model

| Goroutine | Purpose |
|-----------|---------|
| main | Signal handling, context root |
| BidirectionalCopy (2) | stdin→network and network→stdout |
| tunnel.monitor | Watches SSH connection health |
| scanner workers (≤100) | Concurrent port probes |
| server accept loop | Connection dispatch |
| server per-connection | One goroutine per client (with `-k`) |

All goroutines respect `context.Context` for cancellation.
`sync.WaitGroup` ensures no goroutine leaks on shutdown.

## Authentication Chain

```
Explicit flags?
  ├─ --ssh-key PATH     → load PEM, prompt passphrase if encrypted
  ├─ --ssh-agent        → connect to SSH_AUTH_SOCK
  └─ --ssh-password     → interactive prompt via x/term
         │
         ▼  (if nothing explicit)
     Default probing:
       1. SSH agent (SSH_AUTH_SOCK)
       2. ~/.ssh/id_ed25519
       3. ~/.ssh/id_rsa
       4. ~/.ssh/id_ecdsa
```

## Error Strategy

* Every public function returns `error`.
* Errors are wrapped with `fmt.Errorf("context: %w", err)` for stack traces.
* `bail()` never happens – the top-level `main` prints and exits.
* Tunnel / network errors include the remote address for actionable diagnostics.

## Buffer Sizing

`util.DefaultBufSize = 32 KiB` – standard for `io.Copy`.
Go's `io.Copy` uses `io.ReaderFrom` / `io.WriterTo` when available,
falling back to a 32 KiB intermediate buffer.

## Windows Considerations

* Go produces a clean PE binary with standard imports.
* `resource/resource.json` adds FileDescription / CompanyName via goversioninfo.
* Exec uses `cmd.exe /C` for `-c` and direct path for `-e`.
* Named-pipe SSH agent (`\\.\pipe\openssh-ssh-agent`) requires external
  support (e.g. `SSH_AUTH_SOCK` set by Git for Windows).  Key-based and
  password auth work natively.

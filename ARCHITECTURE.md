# GoNC – Architecture

## Design Goals

1. **Single static binary** – zero runtime dependencies.
2. **Cross-platform** – identical behaviour on Linux, macOS, and Windows.
3. **Minimal dependency surface** – only `x/crypto`, `x/term`, and `pflag`.
4. **Capability-centric composable architecture** – transport, capability,
   session, and orchestration layers are cleanly separated.
5. **Testability** – every subsystem is unit-testable in isolation.

## Layered Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  CLI Layer         cmd/root.go                              │
│  Parse flags → validate → core.Build(cfg) → mode.Run(ctx)  │
└─────────────────────────┬───────────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────────┐
│  Orchestration     internal/core/                           │
│  Build(cfg) dispatches to the right Mode:                   │
│    ConnectMode │ ListenMode │ ScanMode │ ReverseTunnelMode  │
│  Each mode composes a Transport + Capability + Session      │
└──────┬──────────────────┬───────────────────────────────────┘
       │                  │
┌──────▼──────┐   ┌───────▼────────┐   ┌─────────────────────┐
│ Transport   │   │ Capability     │   │ Session             │
│ internal/   │   │ internal/      │   │ internal/session/   │
│ transport/  │   │ capability/    │   │                     │
│             │   │                │   │ Binds net.Conn +    │
│ TCPDialer   │   │ Relay (stdio)  │   │ stdin/stdout +      │
│ UDPDialer   │   │ Exec (child)   │   │ logger into one     │
│ SSHDialer   │   │                │   │ struct for caps     │
└──────┬──────┘   └────────────────┘   └─────────────────────┘
       │
┌──────▼──────────────────────────────────────────────────────┐
│  Tunnel Layer      tunnel/                                  │
│  SSHTunnel (forward) │ ReverseTunnel (ssh -R equivalent)    │
│  Auth, listener, forwarder, health, dial subsystems         │
└─────────────────────────────────────────────────────────────┘
       │
┌──────▼──────────────────────────────────────────────────────┐
│  Support Packages                                           │
│  config/          Config struct, validation, env loader     │
│  util/            BidirectionalCopy, logger, network, pool  │
│  internal/errors/ NetworkError, SSHError, ConfigError       │
│  internal/retry/  Exponential backoff, circuit breaker      │
│  internal/metrics/Lock-free atomic counters, Snapshot/JSON  │
└─────────────────────────────────────────────────────────────┘
```

## Package Map

```
main.go                        Entry point: signal context → cmd.Execute()
  ↓
cmd/
  └─ root.go                   CLI flags → Config → core.Build() → mode.Run()
  ↓
internal/core/                  Orchestration layer
  ├─ mode.go                   Mode interface
  ├─ builder.go                Build(cfg, logger) → Mode (single dispatch point)
  ├─ connect.go                ConnectMode: Dialer + Capability
  ├─ listen.go                 ListenMode: TCP/UDP accept → Capability per conn
  ├─ scan.go                   ScanMode: concurrent port probing + ScanPorts()
  └─ reverse.go                ReverseTunnelMode: wraps tunnel.ReverseTunnel
  ↓
internal/transport/             How data moves
  ├─ transport.go              Dialer interface
  ├─ tcp.go                    TCPDialer (plain TCP, optional source port)
  ├─ udp.go                    UDPDialer (plain UDP, optional source port)
  └─ ssh.go                    SSHDialer (lazy-connect SSH tunnel wrapper)
  ↓
internal/capability/            What happens over a connection
  ├─ capability.go             Capability interface
  ├─ relay.go                  Relay: stdin/stdout ↔ connection
  └─ exec.go                   Exec: wire connection to child process stdio
  ↓
internal/session/               Connection lifecycle
  └─ session.go                Session: Conn + Stdin + Stdout + Logger
  ↓
config/                         Configuration
  ├─ config.go                 Config struct, Validate(), Parse helpers
  ├─ defaults.go               Centralised default constants
  └─ loader.go                 GONC_* environment variable loader
  ↓
tunnel/                         SSH tunnel implementations
  ├─ tunnel.go                 Tunnel interface (Connect, Dial, Close, IsAlive)
  ├─ ssh.go                    SSH forward tunnel (x/crypto/ssh)
  ├─ auth.go                   Auth methods (key, agent, password, kb-interactive)
  ├─ reverse_tunnel.go         Reverse tunnel lifecycle: Start / Wait / Close
  ├─ reverse_listener.go       Custom forwarded-tcpip SSH listener
  ├─ reverse_forwarder.go      Connection bridging with byte metrics
  ├─ reverse_health.go         Keepalive, reconnection, sleepCtx
  ├─ reverse_dial.go           SSH dial, gateway-ports validation
  └─ manager.go                Health monitoring goroutine
  ↓
internal/
  ├─ errors/errors.go          NetworkError, SSHError, ConfigError, sentinels
  ├─ retry/backoff.go          Exponential backoff with jitter
  ├─ retry/circuit_breaker.go  Closed → Open → Half-Open state machine
  └─ metrics/metrics.go        Lock-free atomic counters, Snapshot, JSON
  ↓
util/
  ├─ io.go                     BidirectionalCopy (goroutine-safe, context-aware)
  ├─ network.go                Address formatting, DNS, free-port finder
  ├─ logger.go                 Levelled stderr logger with timestamps + prefixes
  └─ pool.go                   sync.Pool byte buffer reuse
```

## Key Interfaces

### Transport — `internal/transport.Dialer`

```go
type Dialer interface {
    Dial(ctx context.Context, network, address string) (net.Conn, error)
    Close() error
}
```

Implementations: `TCPDialer`, `UDPDialer`, `SSHDialer`.
SSHDialer lazy-connects on first Dial; Close tears down the tunnel.

### Capability — `internal/capability.Capability`

```go
type Capability interface {
    Handle(ctx context.Context, sess *session.Session) error
}
```

Implementations: `Relay` (stdin/stdout ↔ conn), `Exec` (child process).

### Mode — `internal/core.Mode`

```go
type Mode interface {
    Run(ctx context.Context) error
}
```

Implementations: `ConnectMode`, `ListenMode`, `ScanMode`, `ReverseTunnelMode`.

## Data Flow

### Direct Connection (ConnectMode + Relay)

```
stdin ──▶ ┌──────────┐        ┌─────────────┐
          │ TCPDialer│──TCP──▶│  Remote Host │
stdout ◀──┤  + Relay │◀──────┤             │
          └──────────┘        └─────────────┘
```

### SSH Tunnel Connection (ConnectMode + SSHDialer + Relay)

```
stdin ──▶ ┌──────────┐        ┌────────────┐        ┌─────────────┐
          │ SSHDialer│──SSH──▶│  Gateway    │──TCP──▶│  Destination│
stdout ◀──┤  + Relay │◀──────┤  (bastion)  │◀──────┤             │
          └──────────┘        └────────────┘        └─────────────┘
```

### Reverse SSH Tunnel (ReverseTunnelMode)

```
Remote Client ──▶ ┌─────────────┐        ┌──────────┐        ┌───────────────┐
                  │ SSH Gateway │──SSH──▶│  GoNC    │──TCP──▶│ Local Service │
                  │ (port 9000) │◀──────┤  reverse │◀──────┤ (port 8080)   │
                  └─────────────┘        └──────────┘        └───────────────┘
```

The reverse tunnel uses a custom `listenRemoteForward()` function that
sends a `tcpip-forward` global request to the SSH server, then registers
a handler for all incoming `forwarded-tcpip` channels.  This approach
replaces `ssh.Client.Listen()` because Go's standard library matches
forwarded channels by the exact bind address string — but many public
tunnel services (serveo.net, localhost.run) echo back a different address
than the one requested, causing a silent mismatch that drops every
connection.  The custom handler accepts **all** `forwarded-tcpip` channels
unconditionally, which is correct when only one forward is active.

## Builder Dispatch

The `core.Build(cfg, logger)` function is the **single dispatch point**
replacing the scattered switch/if trees that previously lived across
multiple packages:

```
cfg.ReverseTunnelEnabled? → ReverseTunnelMode
cfg.Listen?               → ListenMode  (TCP or UDP, KeepOpen)
cfg.ZeroIO?               → ScanMode    (concurrent port probe)
default                   → ConnectMode (TCP/UDP, optional SSH)
```

Each mode is composed from:
- A **Transport** (`Dialer`) selected by `buildDialer(cfg)` — TCPDialer,
  UDPDialer, or SSHDialer depending on protocol and tunnel flags.
- A **Capability** selected by `buildCapability(cfg)` — Relay for
  interactive/pipe mode, Exec for `-e`/`-c`.
- A **Session** created at connection time, binding the transport's
  `net.Conn` with stdin/stdout for the capability to operate on.

## Concurrency Model

| Goroutine | Purpose |
|-----------|---------|
| main | Signal handling, context root |
| BidirectionalCopy (2) | stdin→network and network→stdout |
| tunnel.monitor | Watches SSH connection health |
| scanner workers (≤100) | Concurrent port probes |
| ListenMode accept loop | Connection dispatch |
| ListenMode per-connection | One goroutine per client (with `-k`) |
| reverse acceptLoop | Accepts remote connections on SSH listener |
| reverse per-connection | Bridges remote conn ↔ local service |
| reverse keepaliveLoop | Periodic SSH keepalive probes |
| reverse drainMessages | Reads server session stdout/stderr |
| reverse ctx watcher | Closes listener on context cancel |

All goroutines respect `context.Context` for cancellation.
`sync.WaitGroup` ensures no goroutine leaks on shutdown.

## Authentication Chain

```
Explicit flags?
  ├─ --ssh-key PATH     → load PEM, prompt passphrase if encrypted
  ├─ --ssh-agent        → connect to SSH_AUTH_SOCK / Windows named pipe
  └─ --ssh-password     → interactive prompt via x/term
         │
         ▼  (if nothing explicit)
     Default probing:
       1. SSH agent (SSH_AUTH_SOCK on Unix, \\.\pipe\openssh-ssh-agent on Windows)
       2. ~/.ssh/id_ed25519
       3. ~/.ssh/id_rsa
       4. ~/.ssh/id_ecdsa
         │
         ▼  (always for reverse tunnels)
       5. Keyboard-interactive (empty challenge responses)
          └─ Needed by serveo.net, localhost.run, and similar services
```

## Error Strategy

* Every public function returns `error`.
* Domain errors use structured types from `internal/errors`:
  - `NetworkError` — carries Op, Addr, Retryable flag.
  - `SSHError` — carries Op, Host, Port for SSH-specific failures.
  - `ConfigError` — carries Field, Value, Message, Hint for user guidance.
* Sentinel errors (`ErrTunnelClosed`, `ErrNotConnected`, …) enable
  `errors.Is()` checks without string matching.
* `IsRetryable(err)` classifies errors for the retry/backoff system.

## Windows Considerations

* Go produces a clean PE binary with standard imports.
* `resource/resource.json` adds FileDescription / CompanyName via goversioninfo.
* Exec uses `cmd.exe /C` for `-c` and direct path for `-e`.
* SSH agent: GoNC connects to the Windows OpenSSH agent via the named pipe
  `\\.\pipe\openssh-ssh-agent` automatically when `SSH_AUTH_SOCK` is not set.
* Username: when no `user@` prefix is given, GoNC defaults to the current OS
  username (with `DOMAIN\user` prefix stripped), matching `ssh` behaviour.

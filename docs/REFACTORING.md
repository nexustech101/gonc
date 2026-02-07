# GoNC Refactoring Log

> Tracking document for the v1.1.0 refactoring cycle.
> Branch: `dev` · Base tag: `v1.1.0-dev.1`

---

## 1  Motivation

GoNC reached ~3 170 LOC after the reverse-SSH-tunnel feature landed.
A senior-engineering review identified eight improvement axes:

| # | Area | Pain Point |
|---|------|------------|
| 1 | Architecture | `tunnel/reverse.go` was 644 lines with mixed concerns |
| 2 | Error handling | `fmt.Errorf` everywhere; no retryability classification |
| 3 | Concurrency | No metrics, no circuit breaker, no buffer reuse |
| 4 | Configuration | CLI-only; no env-var / config-file support |
| 5 | Testing | No benchmarks, no CLI integration tests |
| 6 | Security | Bare `exec` path; no hardening guidelines |
| 7 | UX | No `--dry-run`; help text lacked env-var docs |
| 8 | Performance | Fresh 32 KiB allocations per copy loop |

---

## 2  Changes Delivered

### 2.1  Infrastructure Packages (`internal/`)

| Package | Purpose | Key Types / Functions |
|---------|---------|---------------------|
| `internal/errors` | Domain-specific error types | `NetworkError`, `SSHError`, `ConfigError`, `Wrap()`, `WrapSSH()`, sentinels |
| `internal/retry` | Exponential backoff + circuit breaker | `Backoff.Do()`, `PermanentError`, `CircuitBreaker.Execute()` |
| `internal/metrics` | Lock-free runtime counters | `Collector` (nil-safe), `Snapshot()`, `JSON()` |

**Design decisions**:
- `internal/` prevents accidental import from external modules.
- `Collector` uses `sync/atomic` for counters, `sync.RWMutex` only for
  timestamps — benchmarks show ~3 ns/op for counter updates.
- `Backoff.Do()` takes a `func(attempt int) error` and accepts
  `PermanentError` to signal non-retryable failures, decoupling retry
  logic from error classification.
- Circuit breaker uses Closed → Open → Half-Open state machine with
  configurable thresholds.

### 2.2  File Split – `tunnel/reverse.go`

The 644-line monolith was split into five focused files:

| File | Responsibility | ~Lines |
|------|----------------|--------|
| `reverse_tunnel.go` | Lifecycle: `Start`, `Wait`, `Close`, `acceptLoop` | 200 |
| `reverse_listener.go` | Custom SSH `forwarded-tcpip` listener | 150 |
| `reverse_forwarder.go` | Connection bridging with byte counts | 70 |
| `reverse_health.go` | Keepalive loop, reconnection, `sleepCtx` | 110 |
| `reverse_dial.go` | SSH dial, gateway-ports validation, message drain | 110 |

**Design decisions**:
- All files remain in `package tunnel` — Go allows methods on a type to
  be spread across files within the same package.
- `bridgeConns()` now returns `(aToB, bToA int64)` for metrics.
- `NewReverseTunnel()` accepts `*metrics.Collector` (nil = no-op).

### 2.3  Configuration

| File | What it does |
|------|-------------|
| `config/defaults.go` | Centralized constants (`DefaultSSHPort`, `DefaultScanTimeout`, …) |
| `config/loader.go` | `LoadFromEnv(cfg)` — reads `GONC_*` environment variables |
| `config/config.go` | `Validate()` now returns `*ConfigError` with `Field`, `Value`, `Hint` |

**Precedence**: CLI flags > Environment variables > Defaults.

Supported env vars: `GONC_HOST`, `GONC_PORT`, `GONC_LISTEN`, `GONC_UDP`,
`GONC_TUNNEL`, `GONC_SSH_KEY`, `GONC_REVERSE_TUNNEL`, `GONC_REMOTE_PORT`, …

### 2.4  Logger & Buffer Pool

- **Logger** (`util/logger.go`): Added `[INF]` / `[WRN]` / `[VRB]` / `[DBG]` /
  `[ERR]` prefixes, optional `HH:MM:SS.mmm` timestamps (auto-enabled in
  debug mode), and a `Warn()` level.
- **BufPool** (`util/pool.go`): `sync.Pool`-based 32 KiB buffer reuse
  via `GetBuf()` / `PutBuf()`.

### 2.5  CLI (`cmd/root.go`)

- `--dry-run` flag validates config and exits without executing.
- `config.LoadFromEnv()` called before flag parsing.
- Help text now documents environment variables and `--dry-run`.

### 2.6  Error Wiring

- `tunnel/ssh.go`: `Connect()` / `Dial()` now return `ncerr.WrapSSH` /
  `ncerr.ErrNotConnected` instead of bare `fmt.Errorf`.
- `netcat/scanner.go`: Uses `config.DefaultScanTimeout` /
  `config.DefaultMaxConcurrentScans`.

---

## 3  Test Coverage

| Package | Unit Tests | Benchmarks |
|---------|-----------|------------|
| `internal/errors` | ✅ errors_test.go | — |
| `internal/retry` | ✅ backoff_test.go, circuit_breaker_test.go | ✅ bench_test.go |
| `internal/metrics` | ✅ metrics_test.go | ✅ bench_test.go |
| `config` | ✅ config_test.go, loader_test.go, validator_test.go | — |
| `util` | ✅ logger_test.go, io_test.go, network_test.go | ✅ bench_test.go |
| `tunnel` | ✅ reverse_test.go, auth_test.go | ✅ bench_test.go |
| `netcat` | ✅ client_test.go, server_test.go, scanner_test.go | ✅ bench_test.go |
| `cmd` | ✅ root_test.go | — |

Run all: `go test ./...`
Run benchmarks: `go test -bench=. -run=^$ ./...`

---

## 4  Benchmark Highlights

```
BenchmarkCollector_ConnectionOpen-12     173M     3.4 ns/op
BenchmarkCollector_BytesSent-12          364M     1.6 ns/op
BenchmarkNilCollector-12                 453M     1.3 ns/op
BenchmarkBackoff_ImmediateSuccess-12     241M     2.4 ns/op
BenchmarkCircuitBreaker_ClosedPath-12     ~fast   (mutex fast path)
BenchmarkBufPool/pool-12                  63M     8.4 ns/op
```

---

## 5  Migration Notes

- **No breaking CLI changes** — all existing flags work identically.
- **`NewReverseTunnel` signature changed** — now requires a 3rd param
  `*metrics.Collector` (pass `nil` for no-op).
- **Error types changed** — `Validate()` returns `*ConfigError` instead
  of `*fmt.wrapError`.  Code using `err != nil` is unaffected; code
  inspecting error strings may need adjustment.

---

## 6  Future Work

| Priority | Item | Notes |
|----------|------|-------|
| High | Wire `BufPool` into `BidirectionalCopy` | Replace `io.Copy` with pool-backed loop |
| High | `errgroup` for goroutine lifecycle | Cleaner shutdown, error propagation |
| Medium | SOCKS5 proxy mode | Requested in README roadmap |
| Medium | Connection pool for SSH channels | Reduce handshake overhead |
| Medium | Shell completions (bash/zsh/fish) | Requires Cobra or manual generator |
| Low | YAML/TOML config file support | Add `gopkg.in/yaml.v3` |
| Low | pprof / expvar endpoint | For long-running tunnel processes |
| Low | `goleak` tests | Detect goroutine leaks |

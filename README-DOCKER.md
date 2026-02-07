# GoNC – Docker Guide

Build, test, and run GoNC entirely within Docker containers.

---

## Quick Reference

| Task | Command |
|------|---------|
| Run tests | `docker compose --profile test up --build test` |
| Build all binaries | `docker compose --profile build up --build build` |
| Integration tests | `docker compose --profile integration up --build` |
| Run interactively | `docker compose --profile run run gonc <host> <port>` |
| Production image | `docker compose -f docker-compose.prod.yaml build` |
| Extract binaries | `docker buildx build --target dist --output ./dist .` |

---

## Dockerfile Stages

The multi-stage [Dockerfile](Dockerfile) has five stages:

```
┌──────┐    ┌──────┐    ┌─────────┐    ┌───────┐    ┌──────┐
│ deps │───▶│ test │    │ builder │───▶│ final │    │ dist │
└──────┘    └──────┘    └─────────┘    └───────┘    └──────┘
  go mod     vet +        cross-       Alpine +      scratch
  download   test -race   compile      gonc binary   all bins
```

| Stage | Purpose |
|-------|---------|
| `deps` | Download Go modules (layer-cached on `go.mod` + `go.sum`) |
| `test` | `go vet` + `go test -race` with coverage report |
| `builder` | Cross-compile static binaries for 5 platform/arch targets |
| `final` | Minimal Alpine image with the Linux amd64 binary (~12 MB) |
| `dist` | `FROM scratch` – extract all binaries to the host filesystem |

---

## Usage

### 1. Run Unit Tests

```bash
docker compose --profile test up --build test
```

Tests run with `-race` and produce a coverage summary. Source code is
mounted so you can iterate without rebuilding the image.

### 2. Build Cross-Platform Binaries

```bash
docker compose --profile build up --build build
```

Binaries appear in `./dist/`:

```
dist/
├── gonc-linux-amd64
├── gonc-linux-arm64
├── gonc-darwin-amd64
├── gonc-darwin-arm64
├── gonc-windows-amd64.exe
└── SHA256SUMS
```

**Alternative** (BuildKit output, no compose):

```bash
docker buildx build --target dist --output type=local,dest=./dist .
```

### 3. Integration Tests (SSH Tunnel)

```bash
docker compose --profile integration up --build --abort-on-container-exit
```

This starts three containers on an isolated network:

| Container | Role |
|-----------|------|
| `ssh-server` | OpenSSH daemon (user: `testuser`, password: `testpass`) |
| `target-server` | TCP echo service on port 8080 (socat) |
| `integration` | Runs GoNC tests: direct connect, port scan, SSH tunnel |

### 4. Run GoNC Interactively

```bash
# Port scan
docker compose --profile run run gonc -vz example.com 80 443

# Connect through SSH tunnel
docker compose --profile run run gonc -T user@bastion target 5432
```

Or without compose:

```bash
docker run --rm -it $(docker build -q --target final .) -vz example.com 80
```

### 5. Production Deployment

```bash
# Build
docker compose -f docker-compose.prod.yaml build

# Run
docker compose -f docker-compose.prod.yaml run --rm gonc <host> <port>
```

The production compose adds:
- Read-only root filesystem
- Dropped capabilities (only `NET_RAW` retained)
- `no-new-privileges` security option
- Memory limit (64 MB) and CPU limit (0.5)
- Optional SSH key volume mount

---

## Build Arguments

| ARG | Default | Description |
|-----|---------|-------------|
| `GO_VERSION` | `1.22` | Go compiler version |
| `ALPINE_VERSION` | `3.20` | Base image version |
| `VERSION` | `1.0.0` | Embedded binary version string |

Override at build time:

```bash
docker build --build-arg VERSION=2.0.0 --target final -t gonc:2.0.0 .
```

---

## Image Size

| Stage | Size |
|-------|------|
| `deps` | ~500 MB (Go SDK + modules) |
| `test` | ~700 MB (+ build-base for -race) |
| `builder` | ~600 MB (cached, not shipped) |
| **`final`** | **~12 MB** (Alpine + gonc binary) |

The final production image is well under the 20 MB target.

---

## Security

- Static binary – no CGO, no shared libraries
- Non-root user (`gonc`) in the final image
- `no-new-privileges` in production compose
- All capabilities dropped except `NET_RAW`
- Read-only root filesystem (production)
- No shell access in production (override entrypoint blocked by read-only fs)

### Scanning

```bash
# Trivy
docker run --rm aquasec/trivy image gonc:latest

# Docker Scout
docker scout cves gonc:latest
```

---

## CI Integration

The GitHub Actions workflow can use Docker for reproducible builds:

```yaml
- name: Build and test in Docker
  run: |
    docker compose --profile test up --build --abort-on-container-exit test
    docker compose --profile build up --build build

- name: Upload binaries
  uses: actions/upload-artifact@v4
  with:
    name: gonc-binaries
    path: dist/
```

---

## Troubleshooting

| Problem | Solution |
|---------|----------|
| `go mod download` slow | Docker caches the `deps` layer; subsequent builds skip it |
| Tests fail on Windows host | Tests run inside a Linux container – host OS doesn't matter |
| SSH tunnel integration hangs | Check `ssh-server` health: `docker compose ps` |
| Permission denied on `./dist/` | The `build` service creates files as root; `sudo chown -R $USER dist/` |
| Image too large | Ensure you're targeting `final`, not `builder` |

# ═══════════════════════════════════════════════════════════════════════
#  GoNC – Multi-stage Dockerfile
#  Stages:  deps → test → builder → final
# ═══════════════════════════════════════════════════════════════════════

# ── Global build args ─────────────────────────────────────────────────
ARG GO_VERSION=1.22
ARG ALPINE_VERSION=3.20
ARG VERSION=1.0.0

# ======================================================================
#  Stage 1 – deps  (download modules, cached unless go.mod/go.sum change)
# ======================================================================
FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS deps

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# ======================================================================
#  Stage 2 – test  (run unit tests with race detector + coverage)
# ======================================================================
FROM deps AS test

# gcc is needed for -race on Alpine
RUN apk add --no-cache build-base

COPY . .

# Default: run tests with race detector and produce coverage report.
# Override with --target test in docker build or docker-compose.
RUN go vet ./...
RUN go test -race -count=1 -timeout 120s -coverprofile=/tmp/coverage.out ./... \
    && go tool cover -func=/tmp/coverage.out | tail -1

# ======================================================================
#  Stage 3 – builder  (cross-compile static binaries for every platform)
# ======================================================================
FROM deps AS builder

ARG VERSION
ENV VERSION=${VERSION}

COPY . .

# Common ldflags for every target
ENV LDFLAGS="-s -w -X gonc/cmd.version=${VERSION}"

# ── Linux amd64 ──────────────────────────────────────────────────────
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags "${LDFLAGS}" -trimpath \
    -o /out/gonc-linux-amd64 .

# ── Linux arm64 ──────────────────────────────────────────────────────
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
    go build -ldflags "${LDFLAGS}" -trimpath \
    -o /out/gonc-linux-arm64 .

# ── Windows amd64 ────────────────────────────────────────────────────
RUN CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
    go build -ldflags "${LDFLAGS}" -trimpath \
    -o /out/gonc-windows-amd64.exe .

# ── macOS amd64 ──────────────────────────────────────────────────────
RUN CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 \
    go build -ldflags "${LDFLAGS}" -trimpath \
    -o /out/gonc-darwin-amd64 .

# ── macOS arm64 ──────────────────────────────────────────────────────
RUN CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
    go build -ldflags "${LDFLAGS}" -trimpath \
    -o /out/gonc-darwin-arm64 .

# SHA-256 manifest
RUN cd /out && sha256sum * > SHA256SUMS

# ======================================================================
#  Stage 4 – final  (minimal runtime image – Alpine ~5 MB base)
# ======================================================================
FROM alpine:${ALPINE_VERSION} AS final

# ca-certificates for TLS, openssh-client for agent-based auth
RUN apk add --no-cache ca-certificates openssh-client \
    && adduser -D -H -s /sbin/nologin gonc

COPY --from=builder /out/gonc-linux-amd64 /usr/local/bin/gonc
RUN chmod +x /usr/local/bin/gonc

USER gonc

ENTRYPOINT ["gonc"]
CMD ["--help"]

# Re-declare global ARG so it's available after FROM
ARG VERSION

# Metadata
LABEL org.opencontainers.image.title="GoNC" \
      org.opencontainers.image.description="Cross-platform netcat with SSH tunneling" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.source="https://github.com/example/gonc"

# ======================================================================
#  Stage: dist  (extract all cross-compiled binaries)
# ======================================================================
FROM scratch AS dist
COPY --from=builder /out/ /

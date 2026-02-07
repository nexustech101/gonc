BINARY  := gonc
VERSION := $(shell git describe --tags 2>/dev/null || echo "1.1.0-dev")
BUILD   := build
LDFLAGS := -s -w -X gonc/cmd.version=$(VERSION)

.PHONY: all build build-all build-linux build-darwin build-windows
.PHONY: test bench coverage lint clean fmt vet check

all: build

# ── Build ─────────────────────────────────────────────────────────

build:
	@mkdir -p $(BUILD)
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD)/$(BINARY) .

build-all: build-linux build-darwin build-windows

build-linux:
	@mkdir -p $(BUILD)
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD)/$(BINARY)-linux-amd64 .
	GOOS=linux   GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD)/$(BINARY)-linux-arm64 .

build-darwin:
	@mkdir -p $(BUILD)
	GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD)/$(BINARY)-darwin-amd64 .
	GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD)/$(BINARY)-darwin-arm64 .

build-windows:
	@mkdir -p $(BUILD)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD)/$(BINARY)-windows-amd64.exe .

# ── Quality ───────────────────────────────────────────────────────

test:
	go test -race -count=1 -timeout 60s ./...

bench:
	go test -bench=. -benchmem -run=^$$ -timeout 120s ./...

coverage:
	go test -race -coverprofile=coverage.out -timeout 60s ./...
	go tool cover -func=coverage.out
	@echo "──────────────────────────────────"
	@echo "HTML report: go tool cover -html=coverage.out"

lint:
	golangci-lint run ./...

fmt:
	gofmt -s -w .
	goimports -w .

vet:
	go vet ./...

check: vet test
	@echo "✓ All checks passed"

# ── Housekeeping ──────────────────────────────────────────────────

clean:
	rm -rf $(BUILD) coverage.out

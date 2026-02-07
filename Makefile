BINARY  := gonc
VERSION := 1.0.0
BUILD   := build
LDFLAGS := -s -w -X gonc/cmd.version=$(VERSION)

.PHONY: all build build-all build-linux build-darwin build-windows
.PHONY: test lint clean fmt vet

all: build

# ── Build ─────────────────────────────────────────────────────────

build:
	@mkdir -p $(BUILD)
	go build -ldflags "$(LDFLAGS)" -o $(BUILD)/$(BINARY) .

build-all: build-linux build-darwin build-windows

build-linux:
	@mkdir -p $(BUILD)
	GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BUILD)/$(BINARY)-linux-amd64 .
	GOOS=linux   GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BUILD)/$(BINARY)-linux-arm64 .

build-darwin:
	@mkdir -p $(BUILD)
	GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BUILD)/$(BINARY)-darwin-amd64 .
	GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BUILD)/$(BINARY)-darwin-arm64 .

build-windows:
	@mkdir -p $(BUILD)
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BUILD)/$(BINARY)-windows-amd64.exe .

# ── Quality ───────────────────────────────────────────────────────

test:
	go test -race -count=1 -timeout 60s ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -s -w .
	goimports -w .

vet:
	go vet ./...

# ── Housekeeping ──────────────────────────────────────────────────

clean:
	rm -rf $(BUILD)

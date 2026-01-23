# Coders Go CLI Makefile

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION)"
BINARY := coders
BUILD_DIR := bin

# Default target
.PHONY: all
all: build

# Build for current platform
.PHONY: build
build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/coders

# Run the TUI
.PHONY: run
run: build
	./$(BUILD_DIR)/$(BINARY) tui

# Run the list command
.PHONY: list
list: build
	./$(BUILD_DIR)/$(BINARY) list

# Build for all platforms
.PHONY: build-all
build-all: build-darwin-arm64 build-darwin-amd64 build-linux-amd64 build-linux-arm64

.PHONY: build-darwin-arm64
build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-arm64 ./cmd/coders

.PHONY: build-darwin-amd64
build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-amd64 ./cmd/coders

.PHONY: build-linux-amd64
build-linux-amd64:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-amd64 ./cmd/coders

.PHONY: build-linux-arm64
build-linux-arm64:
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-arm64 ./cmd/coders

# Clean build artifacts
.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)

# Run tests
.PHONY: test
test:
	go test -v ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Format code
.PHONY: fmt
fmt:
	go fmt ./...

# Lint code
.PHONY: lint
lint:
	golangci-lint run

# Tidy dependencies
.PHONY: tidy
tidy:
	go mod tidy

# Install locally
.PHONY: install
install: build
	cp $(BUILD_DIR)/$(BINARY) $(GOPATH)/bin/$(BINARY)

# Development: watch and rebuild
.PHONY: watch
watch:
	@which watchexec > /dev/null || (echo "Install watchexec: cargo install watchexec-cli" && exit 1)
	watchexec -e go -r -- make run

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build         - Build for current platform"
	@echo "  build-all     - Build for all platforms"
	@echo "  run           - Build and run the TUI"
	@echo "  list          - Build and run the list command"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  fmt           - Format code"
	@echo "  lint          - Lint code"
	@echo "  tidy          - Tidy dependencies"
	@echo "  clean         - Remove build artifacts"
	@echo "  install       - Install to GOPATH/bin"
	@echo "  watch         - Watch for changes and rebuild"

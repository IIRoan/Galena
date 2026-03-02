# galena Makefile

# Build variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -ldflags "-X github.com/iiroan/galena/cmd/galena/cmd.Version=$(VERSION) \
	-X github.com/iiroan/galena/cmd/galena/cmd.Commit=$(COMMIT) \
	-X github.com/iiroan/galena/cmd/galena/cmd.BuildDate=$(BUILD_DATE)"

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOMOD := $(GOCMD) mod
BINARY_NAME := galena
BUILD_BINARY_NAME := galena-build

# Installation paths
PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin

.PHONY: all build clean test install uninstall fmt lint help

## Default target
all: build

## Build galena and galena-build binaries
build:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) ./cmd/galena/
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_BINARY_NAME) ./cmd/galena-build/

## Build for multiple platforms
build-all: build-linux build-darwin

build-linux:
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-linux-amd64 ./cmd/galena/
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_BINARY_NAME)-linux-amd64 ./cmd/galena-build/
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-linux-arm64 ./cmd/galena/
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_BINARY_NAME)-linux-arm64 ./cmd/galena-build/

build-darwin:
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin-amd64 ./cmd/galena/
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_BINARY_NAME)-darwin-amd64 ./cmd/galena-build/
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin-arm64 ./cmd/galena/
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_BINARY_NAME)-darwin-arm64 ./cmd/galena-build/

## Install galena to system
install: build
	install -d $(DESTDIR)$(BINDIR)
	install -m 755 $(BINARY_NAME) $(DESTDIR)$(BINDIR)/$(BINARY_NAME)
	install -m 755 $(BUILD_BINARY_NAME) $(DESTDIR)$(BINDIR)/$(BUILD_BINARY_NAME)

## Uninstall galena from system
uninstall:
	rm -f $(DESTDIR)$(BINDIR)/$(BINARY_NAME)
	rm -f $(DESTDIR)$(BINDIR)/$(BUILD_BINARY_NAME)

## Clean build artifacts
clean:
	rm -f $(BINARY_NAME) $(BINARY_NAME)-*
	rm -f $(BUILD_BINARY_NAME) $(BUILD_BINARY_NAME)-*
	rm -f build-manifest.json sbom.spdx.json
	rm -rf output/

## Run tests
test:
	$(GOTEST) -v ./...

## Format code
fmt:
	$(GOCMD) fmt ./...

## Run linter
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping"; \
	fi

## Update dependencies
deps:
	$(GOMOD) tidy
	$(GOMOD) verify

## Run galena in development
run: build
	./$(BINARY_NAME) $(ARGS)

## Show version
version: build
	./$(BINARY_NAME) version

## Show project status
status: build
	./$(BUILD_BINARY_NAME) status

## Build container image (via galena)
image: build
	./$(BUILD_BINARY_NAME) build

## Build disk image (via galena)
disk: build
	./$(BUILD_BINARY_NAME) disk qcow2

## Run VM (via galena)
vm: build
	./$(BUILD_BINARY_NAME) vm run

## Validate project
validate: build
	./$(BUILD_BINARY_NAME) validate

## Show help
help:
	@echo "galena - OCI-native OS image build tool"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build       Build galena and galena-build binaries"
	@echo "  install     Install galena and galena-build to $(BINDIR)"
	@echo "  uninstall   Remove galena and galena-build from $(BINDIR)"
	@echo "  clean       Remove build artifacts"
	@echo "  test        Run tests"
	@echo "  fmt         Format Go code"
	@echo "  lint        Run linter"
	@echo "  deps        Update dependencies"
	@echo "  version     Show galena (management CLI) version"
	@echo "  status      Show build/project status via galena-build"
	@echo "  image       Build container image via galena-build"
	@echo "  disk        Build disk image (qcow2) via galena-build"
	@echo "  vm          Run VM with built image via galena-build"
	@echo "  validate    Validate project files via galena-build"
	@echo "  help        Show this help"

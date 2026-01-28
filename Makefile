# finctl Makefile

# Build variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -ldflags "-X github.com/finpilot/finctl/cmd/finctl/cmd.Version=$(VERSION) \
	-X github.com/finpilot/finctl/cmd/finctl/cmd.Commit=$(COMMIT) \
	-X github.com/finpilot/finctl/cmd/finctl/cmd.BuildDate=$(BUILD_DATE)"

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOMOD := $(GOCMD) mod
BINARY_NAME := finctl

# Installation paths
PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin

.PHONY: all build clean test install uninstall fmt lint help

## Default target
all: build

## Build the finctl binary
build:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) ./cmd/finctl/

## Build for multiple platforms
build-all: build-linux build-darwin

build-linux:
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-linux-amd64 ./cmd/finctl/
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-linux-arm64 ./cmd/finctl/

build-darwin:
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin-amd64 ./cmd/finctl/
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin-arm64 ./cmd/finctl/

## Install finctl to system
install: build
	install -d $(DESTDIR)$(BINDIR)
	install -m 755 $(BINARY_NAME) $(DESTDIR)$(BINDIR)/$(BINARY_NAME)

## Uninstall finctl from system
uninstall:
	rm -f $(DESTDIR)$(BINDIR)/$(BINARY_NAME)

## Clean build artifacts
clean:
	rm -f $(BINARY_NAME) $(BINARY_NAME)-*
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

## Run finctl in development
run: build
	./$(BINARY_NAME) $(ARGS)

## Show version
version: build
	./$(BINARY_NAME) version

## Show project status
status: build
	./$(BINARY_NAME) status

## Build container image (via finctl)
image: build
	./$(BINARY_NAME) build

## Build disk image (via finctl)
disk: build
	./$(BINARY_NAME) disk qcow2

## Run VM (via finctl)
vm: build
	./$(BINARY_NAME) vm run

## Validate project
validate: build
	./$(BINARY_NAME) validate

## Show help
help:
	@echo "finctl - OCI-native OS image build tool"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build       Build the finctl binary"
	@echo "  install     Install finctl to $(BINDIR)"
	@echo "  uninstall   Remove finctl from $(BINDIR)"
	@echo "  clean       Remove build artifacts"
	@echo "  test        Run tests"
	@echo "  fmt         Format Go code"
	@echo "  lint        Run linter"
	@echo "  deps        Update dependencies"
	@echo "  version     Show finctl version"
	@echo "  status      Show project status"
	@echo "  image       Build container image"
	@echo "  disk        Build disk image (qcow2)"
	@echo "  vm          Run VM with built image"
	@echo "  validate    Validate project files"
	@echo "  help        Show this help"

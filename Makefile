# Makefile for C2PA Remover
# Builds both native and WebAssembly versions

# Go compiler
GO := go
GOFLAGS := -v

# Project name and binaries
PROJECT_NAME := c2paremover
NATIVE_BIN := $(PROJECT_NAME)
WASM_BIN := $(PROJECT_NAME).wasm

# Build all targets by default
all: build wasm

# Build native binary
build:
	@echo "Building native binary..."
	$(GO) build $(GOFLAGS) -o $(NATIVE_BIN)

# Build WebAssembly binary
wasm:
	@echo "Building WebAssembly binary..."
	$(GO) build $(GOFLAGS) -tags=wasmer -o $(WASM_BIN)

# Run tests
test:
	@echo "Running tests..."
	$(GO) test $(GOFLAGS) ./...

# Clean up build artifacts
clean:
	@echo "Cleaning up..."
	rm -f $(NATIVE_BIN) $(WASM_BIN)

# Install the native binary
install: build
	@echo "Installing binary..."
	install -m 755 $(NATIVE_BIN) /usr/local/bin/$(NATIVE_BIN)

.PHONY: all build wasm test clean install

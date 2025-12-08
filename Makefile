# aicli Build Makefile
# Cross-platform builds for macOS ARM, Linux AMD64, Linux ARM64, Windows

BINARY_NAME=aicli
VERSION=$(shell cat VERSION 2>/dev/null || echo "0.0.1")
BUILD_DIR=dist
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"

# Build targets
.PHONY: all clean darwin-arm64 linux-amd64 linux-arm64 windows-amd64 installers

all: darwin-arm64 linux-amd64 linux-arm64 windows-amd64

# macOS ARM64 (Apple Silicon)
darwin-arm64:
	@echo "Building for macOS ARM64..."
	@mkdir -p $(BUILD_DIR)/darwin-arm64
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/darwin-arm64/$(BINARY_NAME) .
	@cd $(BUILD_DIR)/darwin-arm64 && zip -q ../$(BINARY_NAME)-darwin-arm64.zip $(BINARY_NAME)
	@echo "Created $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64.zip"

# Linux AMD64 (Elementary OS, Ubuntu, etc.)
linux-amd64:
	@echo "Building for Linux AMD64..."
	@mkdir -p $(BUILD_DIR)/linux-amd64
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/linux-amd64/$(BINARY_NAME) .
	@cd $(BUILD_DIR)/linux-amd64 && tar czf ../$(BINARY_NAME)-linux-amd64.tar.gz $(BINARY_NAME)
	@echo "Created $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64.tar.gz"

# Linux ARM64 (NVIDIA GB10 Grace, Raspberry Pi 4, etc.)
linux-arm64:
	@echo "Building for Linux ARM64 (GB10 Grace)..."
	@mkdir -p $(BUILD_DIR)/linux-arm64
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/linux-arm64/$(BINARY_NAME) .
	@cd $(BUILD_DIR)/linux-arm64 && tar czf ../$(BINARY_NAME)-linux-arm64.tar.gz $(BINARY_NAME)
	@echo "Created $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64.tar.gz"

# Windows AMD64
windows-amd64:
	@echo "Building for Windows AMD64..."
	@mkdir -p $(BUILD_DIR)/windows-amd64
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/windows-amd64/$(BINARY_NAME).exe .
	@cd $(BUILD_DIR)/windows-amd64 && zip -q ../$(BINARY_NAME)-windows-amd64.zip $(BINARY_NAME).exe
	@echo "Created $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.zip"

# Create installer scripts
installers:
	@echo "Creating installer scripts..."
	@cp scripts/install.sh $(BUILD_DIR)/ 2>/dev/null || true
	@cp scripts/install.ps1 $(BUILD_DIR)/ 2>/dev/null || true

clean:
	@rm -rf $(BUILD_DIR)
	@echo "Cleaned build directory"

# Build for current platform only
local:
	go build $(LDFLAGS) -o $(BINARY_NAME) .

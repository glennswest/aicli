# aicli Build Makefile
# Cross-platform builds for macOS ARM, Linux AMD64, Linux ARM64, Windows, RHEL/Fedora

BINARY_NAME=aicli
VERSION=$(shell cat VERSION 2>/dev/null || echo "0.0.1")
BUILD_DIR=dist
RPM_DIR=rpm
INSTALL_DIR=$(HOME)/bin
GO=$(shell which go 2>/dev/null || echo "/usr/local/go/bin/go")
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"

# Build targets
.PHONY: all clean darwin-arm64 linux-amd64 linux-arm64 windows-amd64 rpm installers dev bump-patch bump-minor bump-major

all: darwin-arm64 linux-amd64 linux-arm64 windows-amd64 rpm

# macOS ARM64 (Apple Silicon)
darwin-arm64:
	@echo "Building for macOS ARM64..."
	@mkdir -p $(BUILD_DIR)/darwin-arm64
	GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/darwin-arm64/$(BINARY_NAME) .
	@cd $(BUILD_DIR)/darwin-arm64 && zip -q ../$(BINARY_NAME)-darwin-arm64.zip $(BINARY_NAME)
	@echo "Created $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64.zip"

# Linux AMD64 (Elementary OS, Ubuntu, etc.)
linux-amd64:
	@echo "Building for Linux AMD64..."
	@mkdir -p $(BUILD_DIR)/linux-amd64
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/linux-amd64/$(BINARY_NAME) .
	@cd $(BUILD_DIR)/linux-amd64 && tar czf ../$(BINARY_NAME)-linux-amd64.tar.gz $(BINARY_NAME)
	@echo "Created $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64.tar.gz"

# Linux ARM64 (NVIDIA GB10 Grace, Raspberry Pi 4, etc.)
linux-arm64:
	@echo "Building for Linux ARM64 (GB10 Grace)..."
	@mkdir -p $(BUILD_DIR)/linux-arm64
	GOOS=linux GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/linux-arm64/$(BINARY_NAME) .
	@cd $(BUILD_DIR)/linux-arm64 && tar czf ../$(BINARY_NAME)-linux-arm64.tar.gz $(BINARY_NAME)
	@echo "Created $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64.tar.gz"

# Windows AMD64
windows-amd64:
	@echo "Building for Windows AMD64..."
	@mkdir -p $(BUILD_DIR)/windows-amd64
	GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/windows-amd64/$(BINARY_NAME).exe .
	@cd $(BUILD_DIR)/windows-amd64 && zip -q ../$(BINARY_NAME)-windows-amd64.zip $(BINARY_NAME).exe
	@echo "Created $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.zip"

# RPM for RHEL/Fedora (requires rpmbuild)
rpm: linux-amd64
	@echo "Building RPM for RHEL/Fedora..."
	@mkdir -p $(RPM_DIR)/{BUILD,RPMS,SOURCES,SPECS,SRPMS}
	@cp $(BUILD_DIR)/linux-amd64/$(BINARY_NAME) $(RPM_DIR)/SOURCES/
	@rpmbuild --define "_topdir $(CURDIR)/$(RPM_DIR)" --target x86_64 -bb $(RPM_DIR)/SPECS/$(BINARY_NAME).spec
	@cp $(RPM_DIR)/RPMS/x86_64/$(BINARY_NAME)-$(VERSION)-1.x86_64.rpm $(BUILD_DIR)/
	@echo "Created $(BUILD_DIR)/$(BINARY_NAME)-$(VERSION)-1.x86_64.rpm"

# Create installer scripts
installers:
	@echo "Creating installer scripts..."
	@cp scripts/install.sh $(BUILD_DIR)/ 2>/dev/null || true
	@cp scripts/install.ps1 $(BUILD_DIR)/ 2>/dev/null || true

clean:
	@rm -rf $(BUILD_DIR) $(RPM_DIR)/BUILD $(RPM_DIR)/RPMS $(RPM_DIR)/SRPMS
	@echo "Cleaned build directory"

# Build for current platform only
local:
	$(GO) build $(LDFLAGS) -o $(BINARY_NAME) .

# Development build: bump patch version, build, and install to ~/bin
dev: bump-patch
	@echo "Building aicli v$$(cat VERSION) for local development..."
	$(GO) build -ldflags "-s -w -X main.version=$$(cat VERSION)" -o $(INSTALL_DIR)/$(BINARY_NAME) .
	@echo "Installed $(INSTALL_DIR)/$(BINARY_NAME) v$$(cat VERSION)"

# Version bumping utilities
bump-patch:
	@V=$$(cat VERSION); \
	MAJOR=$$(echo $$V | cut -d. -f1); \
	MINOR=$$(echo $$V | cut -d. -f2); \
	PATCH=$$(echo $$V | cut -d. -f3); \
	NEWPATCH=$$((PATCH + 1)); \
	echo "$$MAJOR.$$MINOR.$$NEWPATCH" > VERSION; \
	echo "Version bumped: $$V -> $$MAJOR.$$MINOR.$$NEWPATCH"

bump-minor:
	@V=$$(cat VERSION); \
	MAJOR=$$(echo $$V | cut -d. -f1); \
	MINOR=$$(echo $$V | cut -d. -f2); \
	NEWMINOR=$$((MINOR + 1)); \
	echo "$$MAJOR.$$NEWMINOR.0" > VERSION; \
	echo "Version bumped: $$V -> $$MAJOR.$$NEWMINOR.0"

bump-major:
	@V=$$(cat VERSION); \
	MAJOR=$$(echo $$V | cut -d. -f1); \
	NEWMAJOR=$$((MAJOR + 1)); \
	echo "$$NEWMAJOR.0.0" > VERSION; \
	echo "Version bumped: $$V -> $$NEWMAJOR.0.0"

# Quick install without version bump (for testing)
install:
	@echo "Building aicli v$(VERSION)..."
	$(GO) build $(LDFLAGS) -o $(INSTALL_DIR)/$(BINARY_NAME) .
	@echo "Installed $(INSTALL_DIR)/$(BINARY_NAME) v$(VERSION)"

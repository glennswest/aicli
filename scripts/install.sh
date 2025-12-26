#!/bin/bash
# aicli installer for Linux and macOS
# Downloads the latest release from GitHub

set -e

REPO="glennswest/aicli"
BINARY_NAME="aicli"
INSTALL_DIR="${INSTALL_DIR:-$HOME/bin}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() { echo -e "${GREEN}==>${NC} $1"; }
warn() { echo -e "${YELLOW}==>${NC} $1"; }
error() { echo -e "${RED}==>${NC} $1"; exit 1; }

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$OS" in
        darwin) OS="darwin" ;;
        linux) OS="linux" ;;
        *) error "Unsupported OS: $OS" ;;
    esac

    case "$ARCH" in
        x86_64|amd64) ARCH="amd64" ;;
        arm64|aarch64) ARCH="arm64" ;;
        *) error "Unsupported architecture: $ARCH" ;;
    esac

    PLATFORM="${OS}-${ARCH}"

    # Determine file extension
    if [ "$OS" = "darwin" ]; then
        EXT="zip"
    else
        EXT="tar.gz"
    fi
}

# Get latest release version from GitHub
get_latest_version() {
    info "Checking for latest release..."

    if command -v curl &> /dev/null; then
        VERSION=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    elif command -v wget &> /dev/null; then
        VERSION=$(wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    else
        error "curl or wget is required"
    fi

    if [ -z "$VERSION" ]; then
        error "Failed to get latest version"
    fi

    info "Latest version: $VERSION"
}

# Download and install
download_and_install() {
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY_NAME}-${PLATFORM}.${EXT}"
    TMP_DIR=$(mktemp -d)
    trap "rm -rf $TMP_DIR" EXIT

    info "Downloading ${BINARY_NAME}-${PLATFORM}.${EXT}..."

    if command -v curl &> /dev/null; then
        curl -sL "$DOWNLOAD_URL" -o "$TMP_DIR/aicli.$EXT"
    else
        wget -q "$DOWNLOAD_URL" -O "$TMP_DIR/aicli.$EXT"
    fi

    # Extract
    info "Extracting..."
    cd "$TMP_DIR"
    if [ "$EXT" = "zip" ]; then
        unzip -q "aicli.$EXT"
    else
        tar -xzf "aicli.$EXT"
    fi

    # Create install directory if needed
    mkdir -p "$INSTALL_DIR"

    # Install
    info "Installing to $INSTALL_DIR..."
    mv "$BINARY_NAME" "$INSTALL_DIR/"
    chmod +x "$INSTALL_DIR/$BINARY_NAME"

    # Sign on macOS
    if [ "$OS" = "darwin" ]; then
        info "Signing binary for macOS..."
        codesign --force --sign - "$INSTALL_DIR/$BINARY_NAME" 2>/dev/null || true
    fi
}

# Check if install dir is in PATH
check_path() {
    if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
        warn "$INSTALL_DIR is not in your PATH"
        echo ""
        echo "Add this to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
        echo ""
        echo "  export PATH=\"\$HOME/bin:\$PATH\""
        echo ""
    fi
}

# Main
main() {
    echo ""
    echo "  aicli installer"
    echo "  ==============="
    echo ""

    detect_platform
    info "Detected platform: $PLATFORM"

    get_latest_version
    download_and_install
    check_path

    echo ""
    info "Installation complete!"
    echo ""
    echo "  Run 'aicli --init' to create default config"
    echo "  Run 'aicli --help' for usage"
    echo ""
}

main "$@"

#!/bin/bash
# aicli installer for Linux and macOS
# Downloads the latest release from GitHub

set -e

REPO="glennswest/aicli"
BINARY_NAME="aicli"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() { echo -e "${GREEN}==>${NC} $1"; }
warn() { echo -e "${YELLOW}==>${NC} $1"; }
error() { echo -e "${RED}==>${NC} $1"; exit 1; }

# Find best install directory from PATH
find_install_dir() {
    # If INSTALL_DIR is explicitly set, use it
    if [ -n "$INSTALL_DIR" ]; then
        echo "$INSTALL_DIR"
        return
    fi

    # Preferred directories in order of preference
    local preferred_dirs=(
        "/usr/local/bin"
        "/opt/homebrew/bin"
        "$HOME/.local/bin"
        "$HOME/bin"
    )

    # First, check preferred directories that exist and are in PATH
    for dir in "${preferred_dirs[@]}"; do
        if [ -d "$dir" ] && [[ ":$PATH:" == *":$dir:"* ]]; then
            # Check if we can write to it (or use sudo for system dirs)
            if [ -w "$dir" ] || [[ "$dir" == /usr/local/bin ]]; then
                echo "$dir"
                return
            fi
        fi
    done

    # Second, look for any writable directory in PATH
    IFS=':' read -ra path_dirs <<< "$PATH"
    for dir in "${path_dirs[@]}"; do
        # Skip system directories we shouldn't write to directly
        case "$dir" in
            /bin|/usr/bin|/sbin|/usr/sbin) continue ;;
        esac
        if [ -d "$dir" ] && [ -w "$dir" ]; then
            echo "$dir"
            return
        fi
    done

    # Fall back to /usr/local/bin (will need sudo)
    echo "/usr/local/bin"
}

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

    # Determine if we need sudo
    SUDO=""
    if [ ! -w "$INSTALL_DIR" ]; then
        if command -v sudo &> /dev/null; then
            SUDO="sudo"
            info "Need sudo to install to $INSTALL_DIR"
        else
            error "Cannot write to $INSTALL_DIR and sudo is not available"
        fi
    fi

    # Create install directory if needed
    $SUDO mkdir -p "$INSTALL_DIR"

    # Install
    info "Installing to $INSTALL_DIR..."
    $SUDO mv "$BINARY_NAME" "$INSTALL_DIR/"
    $SUDO chmod +x "$INSTALL_DIR/$BINARY_NAME"

    # Sign on macOS
    if [ "$OS" = "darwin" ]; then
        info "Signing binary for macOS..."
        $SUDO codesign --force --sign - "$INSTALL_DIR/$BINARY_NAME" 2>/dev/null || true
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

    # Find best install directory
    INSTALL_DIR=$(find_install_dir)
    info "Install directory: $INSTALL_DIR"

    get_latest_version
    download_and_install

    echo ""
    info "Installation complete!"
    echo ""
    echo "  Run 'aicli --init' to create default config"
    echo "  Run 'aicli --help' for usage"
    echo ""
}

main "$@"

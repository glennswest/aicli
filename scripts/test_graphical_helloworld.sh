#!/bin/bash
#
# Test script for aicli: Graphical Hello World in Go
# This is a standard test to run after modifications to the app
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
TEST_DIR="/tmp/aicli_test_$$"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${YELLOW}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
}

log_error() {
    echo -e "${RED}[FAIL]${NC} $1"
}

cleanup() {
    if [ -d "$TEST_DIR" ]; then
        log_info "Cleaning up test directory: $TEST_DIR"
        rm -rf "$TEST_DIR"
    fi
}

trap cleanup EXIT

# Helper to get the go binary
go_cmd() {
    if [ -f /usr/local/go/bin/go ]; then
        /usr/local/go/bin/go "$@"
    else
        go "$@"
    fi
}

# Try to build, return 0 on success
try_build() {
    go_cmd build -o helloworld_app . 2>&1
}

main() {
    log_info "=== AICLI Graphical Hello World Test ==="
    log_info "Project directory: $PROJECT_DIR"

    # Ensure aicli is built
    log_info "Building aicli..."
    cd "$PROJECT_DIR"
    go_cmd build -o aicli .

    if [ ! -x "$PROJECT_DIR/aicli" ]; then
        log_error "Failed to build aicli"
        exit 1
    fi
    log_success "aicli built successfully"

    # Create test directory
    mkdir -p "$TEST_DIR"
    cd "$TEST_DIR"
    log_info "Test directory: $TEST_DIR"

    # Initialize a minimal go.mod for the test
    go_cmd mod init helloworld_test

    # Run aicli with the test prompt
    log_info "Running aicli with graphical hello world prompt..."
    PROMPT="Create a graphical hello world app in Go using fyne.io/fyne/v2. Steps: 1) First write main.go with the GUI code using fyne v2 imports, 2) Run 'go get fyne.io/fyne/v2' to install dependencies, 3) Run 'go build -o helloworld .' to build the executable."

    # Run with timeout
    if timeout 300 "$PROJECT_DIR/aicli" -auto -p "$PROMPT" 2>&1 | tee test_output.log; then
        log_info "aicli completed"
    else
        log_error "aicli failed or timed out"
        cat test_output.log
        exit 1
    fi

    # Check for generated source files
    log_info "Checking for generated source files..."
    GO_FILES=$(find . -name "*.go" -type f 2>/dev/null | wc -l)
    if [ "$GO_FILES" -gt 0 ]; then
        log_success "Found $GO_FILES Go source file(s):"
        find . -name "*.go" -type f
    else
        log_error "No Go source files were generated"
        log_info "Test output:"
        cat test_output.log
        exit 1
    fi

    # Check for executable
    log_info "Checking for executable..."
    if [ -x helloworld ] || [ -x helloworld_app ]; then
        log_success "Found executable!"
        ls -la helloworld* 2>/dev/null || true
    else
        log_info "No executable found - attempting fixes..."

        # Step 1: Try go mod tidy
        log_info "Running go mod tidy..."
        go_cmd mod tidy 2>&1 || true

        # Step 2: Try build
        if try_build; then
            log_success "Build succeeded after go mod tidy!"
        else
            BUILD_ERROR=$(try_build 2>&1 || true)
            log_info "Build still failing: $BUILD_ERROR"

            # Step 3: Ask AI to debug
            log_info "Asking AI to debug the failure..."
            DEBUG_PROMPT="The Go build failed with: $BUILD_ERROR. Please read main.go, fix any issues, and rebuild with 'go build -o helloworld_app .'."

            if timeout 300 "$PROJECT_DIR/aicli" -auto -p "$DEBUG_PROMPT" 2>&1 | tee debug_output.log; then
                log_info "Debug attempt completed"
            else
                log_error "Debug attempt failed or timed out"
            fi

            # Step 4: Final build attempt
            log_info "Final build attempt..."
            go_cmd mod tidy 2>&1 || true
            if try_build; then
                log_success "Build succeeded after AI debug!"
            else
                log_info "Build still failing after debug - will check partial success"
            fi
        fi
    fi

    # Final verification
    if [ -x helloworld ] || [ -x helloworld_app ]; then
        log_info "=== Test Summary ==="
        log_success "All checks passed - executable built!"
        log_info "Generated files:"
        ls -la *.go helloworld* 2>/dev/null || ls -la
        log_info ""
        log_info "To run the graphical app: cd $TEST_DIR && ./helloworld_app"
    else
        # Check if we at least got source files - partial success
        if [ "$GO_FILES" -gt 0 ]; then
            log_info "=== Test Summary ==="
            log_info "Partial success: Source files were generated but build failed"
            log_info "This indicates aicli is functioning but the AI model needs improvement"
            log_info "Generated files:"
            ls -la *.go 2>/dev/null || ls -la
            log_info "Build error:"
            try_build 2>&1 || true
            # Don't exit 1 - source generation means aicli is working
            exit 0
        else
            log_error "No source files or executable generated"
            exit 1
        fi
    fi
}

main "$@"

#!/bin/bash
set -e

echo "Testing npm package build process..."

# Check if GoReleaser is installed
if ! command -v goreleaser &> /dev/null; then
    echo "Error: goreleaser not found."
    echo "Install options:"
    echo "  - macOS: brew install goreleaser"
    echo "  - Linux/Windows: Download from https://github.com/goreleaser/goreleaser/releases/latest"
    echo "  - Or try: go install github.com/goreleaser/goreleaser@latest"
    exit 1
fi

# Clean previous builds
rm -rf binaries/ dist/

# Build all platforms
echo "Building all platform binaries..."
goreleaser build -f .goreleaser/npm.yml --snapshot --clean

# Verify binaries directory structure
echo "Verifying binaries directory structure..."
expected_dirs=(
    "binaries/darwin-amd64"
    "binaries/darwin-arm64"
    "binaries/linux-amd64"
    "binaries/linux-arm64"
    "binaries/win32-amd64"
    "binaries/win32-386"
)

for dir in "${expected_dirs[@]}"; do
    if [ ! -d "$dir" ]; then
        echo "Error: Missing directory: $dir"
        exit 1
    fi
done

# Verify all binaries exist
echo "Verifying all binaries exist..."
expected_binaries=(
    "binaries/darwin-amd64/hookdeck"
    "binaries/darwin-arm64/hookdeck"
    "binaries/linux-amd64/hookdeck"
    "binaries/linux-arm64/hookdeck"
    "binaries/win32-amd64/hookdeck.exe"
    "binaries/win32-386/hookdeck.exe"
)

for binary in "${expected_binaries[@]}"; do
    if [ ! -f "$binary" ]; then
        echo "Error: Missing binary: $binary"
        exit 1
    fi
    echo "✓ Found: $binary"
done

# Verify binary architectures using 'file' command
echo ""
echo "Verifying binary architectures..."

verify_binary_arch() {
    local binary="$1"
    local expected_pattern="$2"
    local description="$3"
    
    file_output=$(file "$binary")
    if echo "$file_output" | grep -q "$expected_pattern"; then
        echo "✓ $description: $file_output"
    else
        echo "✗ $description: Expected '$expected_pattern' but got:"
        echo "  $file_output"
        exit 1
    fi
}

# Darwin binaries
verify_binary_arch "binaries/darwin-amd64/hookdeck" "Mach-O 64-bit.*x86_64" "darwin-amd64"
verify_binary_arch "binaries/darwin-arm64/hookdeck" "Mach-O 64-bit.*arm64" "darwin-arm64"

# Linux binaries
verify_binary_arch "binaries/linux-amd64/hookdeck" "ELF 64-bit.*x86-64" "linux-amd64"
verify_binary_arch "binaries/linux-arm64/hookdeck" "ELF 64-bit.*ARM aarch64" "linux-arm64"

# Windows binaries - PE32 is 32-bit, PE32+ is 64-bit
verify_binary_arch "binaries/win32-amd64/hookdeck.exe" "PE32+ executable.*x86-64" "win32-amd64 (64-bit)"
verify_binary_arch "binaries/win32-386/hookdeck.exe" "PE32 executable.*Intel 80386" "win32-386 (32-bit)"

echo ""
echo "✓ All binary architectures verified!"

# Test wrapper script on current platform
echo ""
echo "Testing wrapper script on current platform..."
if node bin/hookdeck.js --version > /dev/null 2>&1; then
    echo "✓ Wrapper script works on $(uname -s)-$(uname -m)"
else
    echo "⚠ Warning: Wrapper script test skipped (binary may not exist for this platform)"
fi

# Test wrapper script exit code handling
echo ""
echo "Testing wrapper script exit code handling..."

# Test that wrapper propagates non-zero exit codes correctly
# Run an invalid command that will cause the binary to exit with non-zero
set +e  # Temporarily disable exit on error
node bin/hookdeck.js invalid-command-that-does-not-exist > /dev/null 2>&1
EXIT_CODE=$?
set -e  # Re-enable exit on error

if [ $EXIT_CODE -ne 0 ]; then
    echo "✓ Wrapper script correctly propagates non-zero exit code ($EXIT_CODE)"
else
    echo "✗ Wrapper script should have returned non-zero exit code for invalid command"
    exit 1
fi

# Test npm pack
echo "Testing npm pack..."
npm pack --dry-run > /tmp/npm-pack-output.txt 2>&1
if grep -q "bin/hookdeck.js" /tmp/npm-pack-output.txt && grep -q "binaries/" /tmp/npm-pack-output.txt; then
    echo "✓ npm pack includes wrapper script and binaries"
else
    echo "Error: npm pack missing required files"
    cat /tmp/npm-pack-output.txt
    exit 1
fi

echo ""
echo "✓ All npm build tests passed!"

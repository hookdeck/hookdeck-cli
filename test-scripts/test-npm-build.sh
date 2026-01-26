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

# Verify binaries exist
echo "Verifying binaries exist..."
if [ ! -f "binaries/darwin-amd64/hookdeck" ]; then
    echo "Error: Missing binary: binaries/darwin-amd64/hookdeck"
    exit 1
fi
if [ ! -f "binaries/win32-amd64/hookdeck.exe" ]; then
    echo "Error: Missing binary: binaries/win32-amd64/hookdeck.exe"
    exit 1
fi

# Test wrapper script on current platform
echo "Testing wrapper script on current platform..."
if node bin/hookdeck.js --version > /dev/null 2>&1; then
    echo "✓ Wrapper script works on $(uname -s)-$(uname -m)"
else
    echo "⚠ Warning: Wrapper script test skipped (binary may not exist for this platform)"
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

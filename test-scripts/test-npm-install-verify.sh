#!/bin/bash
# Verify hookdeck-cli npm package (wrapper, binaries, platform binary).
# Single source of truth for npm install verification; run locally or from CI.
# Uses a local install directory only (no global install).
#
# Usage:
#   INSTALL_DIR=/path/to/install ./test-scripts/test-npm-install-verify.sh
#
#   INSTALL_DIR = directory where "npm install hookdeck-cli@<version>" was run
#                 (must contain node_modules/hookdeck-cli)
#
#   PLATFORM = optional; darwin|linux|win32 (auto-detected from uname if not set)
#
# Running locally (from repo root):
#   INSTALL_DIR=$(mktemp -d)
#   (cd "$INSTALL_DIR" && echo '{"name":"verify-test","private":true}' > package.json && npm install hookdeck-cli@latest)
#   INSTALL_DIR="$INSTALL_DIR" ./test-scripts/test-npm-install-verify.sh
#   rm -rf "$INSTALL_DIR"

set -e

if [ -z "$INSTALL_DIR" ]; then
  echo "✗ INSTALL_DIR is required (directory containing node_modules/hookdeck-cli)"
  echo "  Example: INSTALL_DIR=/tmp/hookdeck-cli-test ./test-scripts/test-npm-install-verify.sh"
  exit 1
fi

PACKAGE_DIR="$INSTALL_DIR/node_modules/hookdeck-cli"
if [ ! -d "$PACKAGE_DIR" ]; then
  echo "✗ Package not found at $PACKAGE_DIR"
  echo "  Run 'npm install hookdeck-cli@<version>' inside INSTALL_DIR first."
  exit 1
fi

echo "Verifying hookdeck-cli at $PACKAGE_DIR"
echo ""

echo "Checking wrapper script..."
if [ ! -f "$PACKAGE_DIR/bin/hookdeck.js" ]; then
  echo "✗ Wrapper script not found at $PACKAGE_DIR/bin/hookdeck.js"
  exit 1
fi
echo "✓ Wrapper script found"
echo ""

echo "Running wrapper (version)..."
node "$PACKAGE_DIR/bin/hookdeck.js" --version
echo ""

echo "Running wrapper (--help)..."
node "$PACKAGE_DIR/bin/hookdeck.js" --help > /dev/null
echo "✓ Wrapper script works"
echo ""

echo "Checking for binaries directory..."
if [ ! -d "$PACKAGE_DIR/binaries" ]; then
  echo "✗ Binaries directory not found at $PACKAGE_DIR/binaries"
  exit 1
fi
echo "✓ Binaries directory found"
ls -la "$PACKAGE_DIR/binaries"
echo ""

# Platform: use env if set (e.g. from CI matrix), else detect from uname
if [ -n "$PLATFORM" ]; then
  echo "Checking platform-specific binary (platform: $PLATFORM from env)..."
else
  case "$(uname -s)" in
    Darwin)  PLATFORM=darwin ;;
    Linux)   PLATFORM=linux ;;
    MINGW*|MSYS*|CYGWIN*) PLATFORM=win32 ;;
    *)       echo "✗ Unsupported platform: $(uname -s)" && exit 1 ;;
  esac
  echo "Checking platform-specific binary (platform: $PLATFORM auto-detected)..."
fi

case "$PLATFORM" in
  darwin)
    if [ -f "$PACKAGE_DIR/binaries/darwin-amd64/hookdeck" ] || [ -f "$PACKAGE_DIR/binaries/darwin-arm64/hookdeck" ]; then
      echo "✓ macOS binary found"
    else
      echo "✗ macOS binary not found"
      exit 1
    fi
    ;;
  linux)
    if [ -f "$PACKAGE_DIR/binaries/linux-amd64/hookdeck" ]; then
      echo "✓ Linux binary found"
    else
      echo "✗ Linux binary not found"
      exit 1
    fi
    ;;
  win32)
    if [ -f "$PACKAGE_DIR/binaries/win32-amd64/hookdeck.exe" ]; then
      echo "✓ Windows binary found"
    else
      echo "✗ Windows binary not found"
      exit 1
    fi
    ;;
  *)
    echo "✗ Unknown PLATFORM: $PLATFORM"
    exit 1
    ;;
esac

echo ""
echo "✓ All verification checks passed!"

#!/bin/bash
# Local test: install hookdeck-cli into a controlled directory and verify (no global install).
# Runs the same verification as the test-npm-install CI action.
#
# Usage: ./test-scripts/test-npm-install-local.sh [version]
#   version: optional, e.g. @latest, @beta, or 1.7.1 (default: @latest)
#
# Installs into test-scripts/.install-test/ (gitignored). No side effects on global npm.

set -e

VERSION="${1:-@latest}"
# npm install needs package@version; tags already have @ (e.g. @latest, @beta)
if [[ "$VERSION" == @* ]]; then
  PKG_SPEC="hookdeck-cli${VERSION}"
else
  PKG_SPEC="hookdeck-cli@${VERSION}"
fi
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="$SCRIPT_DIR/.install-test"

echo "Installing ${PKG_SPEC} into $INSTALL_DIR (local only)..."
mkdir -p "$INSTALL_DIR"
cd "$INSTALL_DIR"
# Isolate from repo: npm would otherwise use repo root's package.json
echo '{"name":"hookdeck-cli-install-test","version":"1.0.0","private":true}' > package.json
npm install "$PKG_SPEC"
echo ""

export INSTALL_DIR="$(pwd)"
"$SCRIPT_DIR/test-npm-install-verify.sh"

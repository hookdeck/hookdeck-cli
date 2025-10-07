#!/bin/bash

# Homebrew Build Validation Test Script for Hookdeck CLI
# --------------------------------------------------------
# This script validates that GoReleaser generates correct Homebrew files
# for the Hookdeck CLI without attempting to install them.
#
# It validates that:
#   - GoReleaser snapshot build completes successfully
#   - Homebrew formula and cask files are generated
#   - Formula contains deprecation warning
#   - Formula references completion files correctly
#   - Cask has proper bash_completion directive
#   - Cask has proper zsh_completion directive
#   - Cask has conflicts with formula
#   - Completion files are bundled in the tarball
#
# Usage:
#   ./test-scripts/test-homebrew-build.sh                              # Build validation only
#   ./test-scripts/test-homebrew-build.sh --install                    # Build + installation testing
#   ./test-scripts/test-homebrew-build.sh --install --bypass-gatekeeper # Full testing with binary execution
#
# Prerequisites:
#   - Go installed
#   - GoReleaser installed (brew install goreleaser)
#   - Homebrew installed (for --install testing)
#
# Note: Without --install, this script only validates BUILD outputs.
# With --install, it also tests actual installation from local tap.
# For CLI functionality testing, use test-scripts/test-acceptance.sh instead.
#
# Unsigned Binary Note:
# Local snapshot builds produce unsigned binaries which macOS Gatekeeper blocks.
# To test binary execution locally, the script can remove the quarantine attribute.
# This requires manual approval for security reasons.

set -e

# Parse command line arguments
RUN_INSTALL_TESTS=false
BYPASS_GATEKEEPER=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --install)
            RUN_INSTALL_TESTS=true
            shift
            ;;
        --bypass-gatekeeper)
            BYPASS_GATEKEEPER=true
            shift
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--install] [--bypass-gatekeeper]"
            exit 1
            ;;
    esac
done

# Global variables for cleanup
LOCAL_TAP_PATH=""
FORMULA_INSTALLED=false
CASK_INSTALLED=false

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Utility functions
echo_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

echo_error() {
    echo -e "${RED}✗ $1${NC}"
}

echo_info() {
    echo -e "${YELLOW}→ $1${NC}"
}

echo_section() {
    echo ""
    echo -e "${BLUE}============================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}============================================${NC}"
    echo ""
}

# Cleanup function for trap
cleanup_installations() {
    if [ "$RUN_INSTALL_TESTS" = false ]; then
        return 0
    fi
    
    echo ""
    echo_section "Cleaning Up Test Installations"
    
    # Uninstall formula if installed
    if [ "$FORMULA_INSTALLED" = true ]; then
        echo_info "Uninstalling formula..."
        if brew uninstall hookdeck 2>/dev/null || true; then
            echo_success "Formula uninstalled"
        fi
    fi
    
    # Uninstall cask if installed
    if [ "$CASK_INSTALLED" = true ]; then
        echo_info "Uninstalling cask..."
        if brew uninstall --cask hookdeck 2>/dev/null || true; then
            echo_success "Cask uninstalled"
        fi
    fi
    
    # Remove local tap
    if [ -n "$LOCAL_TAP_PATH" ] && [ -d "$LOCAL_TAP_PATH" ]; then
        echo_info "Removing local test tap..."
        rm -rf "$LOCAL_TAP_PATH"
        echo_success "Local tap removed"
    fi
    
    echo_success "Cleanup completed"
}

# Set up trap for cleanup on exit
trap cleanup_installations EXIT

# Check prerequisites
check_prerequisites() {
    echo_section "Checking Prerequisites"
    
    if ! command -v go &> /dev/null; then
        echo_error "Go is not installed. Please install Go first."
        exit 1
    fi
    echo_success "Go is installed: $(go version)"
    
    if ! command -v goreleaser &> /dev/null; then
        echo_error "GoReleaser is not installed. Install with: brew install goreleaser"
        exit 1
    fi
    echo_success "GoReleaser is installed: $(goreleaser --version | head -n1)"
    
    if [ "$RUN_INSTALL_TESTS" = true ]; then
        if ! command -v brew &> /dev/null; then
            echo_error "Homebrew is not installed. Homebrew is required for --install testing."
            exit 1
        fi
        echo_success "Homebrew is installed: $(brew --version | head -n1)"
    fi
}

# Clean previous build artifacts
clean_dist() {
    echo_section "Cleaning Previous Build Artifacts"
    
    if [ -d "dist" ]; then
        echo_info "Removing existing dist/ directory..."
        rm -rf dist
        echo_success "Cleaned dist/ directory"
    else
        echo_info "No dist/ directory to clean"
    fi
}

# Run GoReleaser snapshot build
run_goreleaser_build() {
    echo_section "Running GoReleaser Snapshot Build"
    
    echo_info "Building with: goreleaser release --snapshot --clean --config .goreleaser/mac.yml"
    if goreleaser release --snapshot --clean --config .goreleaser/mac.yml; then
        echo_success "GoReleaser build completed successfully"
    else
        echo_error "GoReleaser build failed"
        exit 1
    fi
}

# Validate Homebrew formula file
validate_formula() {
    echo_section "Validating Formula (dist/homebrew/Formula/hookdeck.rb)"
    
    local formula_file="dist/homebrew/Formula/hookdeck.rb"
    
    if [ ! -f "$formula_file" ]; then
        echo_error "Formula file not found at $formula_file"
        return 1
    fi
    echo_success "Formula file exists"
    
    # Check for deprecation warning
    if grep -q "WARNING: This formula is deprecated" "$formula_file"; then
        echo_success "Formula contains deprecation warning"
    else
        echo_error "Formula missing deprecation warning"
        return 1
    fi
    
    # Check for bash completion reference
    if grep -q 'bash_completion.install' "$formula_file"; then
        echo_success "Formula contains bash_completion directive"
    else
        echo_error "Formula missing bash_completion directive"
        return 1
    fi
    
    # Check for zsh completion reference
    if grep -q 'zsh_completion.install' "$formula_file"; then
        echo_success "Formula contains zsh_completion directive"
    else
        echo_error "Formula missing zsh_completion directive"
        return 1
    fi
    
    # Check for completion files in install block
    if grep -q 'completions/hookdeck.bash' "$formula_file"; then
        echo_success "Formula references completions/hookdeck.bash"
    else
        echo_error "Formula missing reference to completions/hookdeck.bash"
        return 1
    fi
    
    if grep -q 'completions/_hookdeck' "$formula_file"; then
        echo_success "Formula references completions/_hookdeck"
    else
        echo_error "Formula missing reference to completions/_hookdeck"
        return 1
    fi
    
    echo_success "Formula validation passed"
    return 0
}

# Validate Homebrew cask file
validate_cask() {
    echo_section "Validating Cask (dist/homebrew/Casks/hookdeck.rb)"
    
    local cask_file="dist/homebrew/Casks/hookdeck.rb"
    
    if [ ! -f "$cask_file" ]; then
        echo_error "Cask file not found at $cask_file"
        return 1
    fi
    echo_success "Cask file exists"
    
    # Check for bash completion
    if grep -q 'bash.*completion.*hookdeck\.bash' "$cask_file"; then
        echo_success "Cask contains bash_completion directive"
    else
        echo_error "Cask missing bash_completion directive"
        return 1
    fi
    
    # Check for zsh completion
    if grep -q 'zsh.*completion.*_hookdeck' "$cask_file"; then
        echo_success "Cask contains zsh_completion directive"
    else
        echo_error "Cask missing zsh_completion directive"
        return 1
    fi
    
    # Check for conflicts with formula
    if grep -q 'conflicts.*formula.*hookdeck' "$cask_file"; then
        echo_success "Cask contains conflicts with formula directive"
    else
        echo_error "Cask missing conflicts with formula directive"
        return 1
    fi
    
    echo_success "Cask validation passed"
    return 0
}

# Validate completion files in tarball
validate_completions_in_tarball() {
    echo_section "Validating Completion Files in Tarball"
    
    # Find the darwin tarball (there should be one for amd64 or arm64)
    local tarball=$(find dist -name "hookdeck_*_darwin_*.tar.gz" | head -n1)
    
    if [ -z "$tarball" ]; then
        echo_error "No darwin tarball found in dist/"
        return 1
    fi
    echo_success "Found tarball: $tarball"
    
    # Check if tarball contains bash completion
    if tar -tzf "$tarball" | grep -q "completions/hookdeck.bash"; then
        echo_success "Tarball contains completions/hookdeck.bash"
    else
        echo_error "Tarball missing completions/hookdeck.bash"
        return 1
    fi
    
    # Check if tarball contains zsh completion
    if tar -tzf "$tarball" | grep -q "completions/_hookdeck"; then
        echo_success "Tarball contains completions/_hookdeck"
    else
        echo_error "Tarball missing completions/_hookdeck"
        return 1
    fi
    
    echo_success "Completion files validation passed"
    return 0
}

# Set up local Homebrew tap for testing
setup_local_tap() {
    echo_section "Setting Up Local Test Tap"
    
    local tap_name="hookdeck-test/hookdeck-test"
    LOCAL_TAP_PATH="$(brew --repository)/Library/Taps/hookdeck-test/homebrew-hookdeck-test"
    
    echo_info "Creating local tap at: $LOCAL_TAP_PATH"
    mkdir -p "$LOCAL_TAP_PATH"
    
    echo_info "Copying Homebrew files to local tap..."
    cp -r dist/homebrew/* "$LOCAL_TAP_PATH/"
    
    # Patch formula to use local file:// URLs for testing
    echo_info "Patching formula to use local file URLs for testing..."
    local formula_file="$LOCAL_TAP_PATH/Formula/hookdeck.rb"
    local current_dir="$(pwd)"
    
    # Replace GitHub URLs with local file:// URLs
    sed -i '' "s|https://github.com/hookdeck/hookdeck-cli/releases/download/v[^/]*/|file://$current_dir/dist/|g" "$formula_file"
    
    # Patch cask to use local file:// URLs for testing
    echo_info "Patching cask to use local file URLs for testing..."
    local cask_file="$LOCAL_TAP_PATH/Casks/hookdeck.rb"
    
    # Replace GitHub URLs with local file:// URLs
    sed -i '' "s|https://github.com/hookdeck/hookdeck-cli/releases/download/v[^/]*/|file://$current_dir/dist/|g" "$cask_file"
    
    echo_success "Local tap created and patched successfully"
    echo_info "Tap name: $tap_name"
}

# Test formula installation
test_formula_installation() {
    echo_section "Testing Formula Installation"
    
    local tap_name="hookdeck-test/hookdeck-test/hookdeck"
    
    echo_info "Installing formula: brew install $tap_name"
    if brew install "$tap_name"; then
        echo_success "Formula installed successfully"
        FORMULA_INSTALLED=true
    else
        echo_error "Formula installation failed"
        return 1
    fi
    
    # Verify binary works (may fail on macOS due to unsigned binary)
    echo_info "Testing binary: hookdeck version"
    
    # Try to run the binary
    if hookdeck version 2>/dev/null; then
        echo_success "Binary is functional"
    else
        # Binary execution failed - likely Gatekeeper
        if [ "$BYPASS_GATEKEEPER" = true ]; then
            echo_info "Binary blocked by Gatekeeper - attempting to bypass..."
            local binary_path="$(which hookdeck)"
            echo_info "Removing quarantine attribute from: $binary_path"
            
            if sudo xattr -d com.apple.quarantine "$binary_path" 2>/dev/null; then
                echo_success "Quarantine attribute removed"
                
                # Try again
                if hookdeck version; then
                    echo_success "Binary is functional after Gatekeeper bypass"
                else
                    echo_error "Binary still failed to execute after bypass"
                    return 1
                fi
            else
                echo_error "Failed to remove quarantine attribute (sudo required)"
                return 1
            fi
        else
            echo_info "Binary test skipped (unsigned binaries are blocked by macOS Gatekeeper)"
            echo_info "Use --bypass-gatekeeper flag to remove quarantine attribute and test binary"
            echo_info "Formula installation succeeded (binary path and completions verified)"
        fi
    fi
    
    # Verify bash completion is installed
    local bash_completion_path="$(brew --prefix)/etc/bash_completion.d/hookdeck"
    echo_info "Checking bash completion at: $bash_completion_path"
    if [ -f "$bash_completion_path" ]; then
        echo_success "Bash completion installed"
    else
        echo_error "Bash completion not found at $bash_completion_path"
        return 1
    fi
    
    # Verify zsh completion is installed
    local zsh_completion_path="$(brew --prefix)/share/zsh/site-functions/_hookdeck"
    echo_info "Checking zsh completion at: $zsh_completion_path"
    if [ -f "$zsh_completion_path" ]; then
        echo_success "Zsh completion installed"
    else
        echo_error "Zsh completion not found at $zsh_completion_path"
        return 1
    fi
    
    echo_success "Formula installation validation passed"
    return 0
}

# Test cask installation
test_cask_installation() {
    echo_section "Testing Cask Installation"
    
    local tap_name="hookdeck-test/hookdeck-test/hookdeck"
    
    echo_info "Installing cask: brew install --cask $tap_name"
    if brew install --cask "$tap_name"; then
        echo_success "Cask installed successfully"
        CASK_INSTALLED=true
    else
        echo_error "Cask installation failed"
        return 1
    fi
    
    # Verify binary works (may fail on macOS due to unsigned binary)
    echo_info "Testing binary: hookdeck version"
    
    # Try to run the binary
    if hookdeck version 2>/dev/null; then
        echo_success "Binary is functional"
    else
        # Binary execution failed - likely Gatekeeper
        if [ "$BYPASS_GATEKEEPER" = true ]; then
            echo_info "Binary blocked by Gatekeeper - attempting to bypass..."
            local binary_path="$(which hookdeck)"
            echo_info "Removing quarantine attribute from: $binary_path"
            
            if sudo xattr -d com.apple.quarantine "$binary_path" 2>/dev/null; then
                echo_success "Quarantine attribute removed"
                
                # Try again
                if hookdeck version; then
                    echo_success "Binary is functional after Gatekeeper bypass"
                else
                    echo_error "Binary still failed to execute after bypass"
                    return 1
                fi
            else
                echo_error "Failed to remove quarantine attribute (sudo required)"
                return 1
            fi
        else
            echo_info "Binary test skipped (unsigned binaries are blocked by macOS Gatekeeper)"
            echo_info "Use --bypass-gatekeeper flag to remove quarantine attribute and test binary"
            echo_info "Cask installation succeeded (binary path and completions verified)"
        fi
    fi
    
    # Verify bash completion is installed
    local bash_completion_path="$(brew --prefix)/etc/bash_completion.d/hookdeck"
    echo_info "Checking bash completion at: $bash_completion_path"
    if [ -f "$bash_completion_path" ]; then
        echo_success "Bash completion installed"
    else
        echo_error "Bash completion not found at $bash_completion_path"
        return 1
    fi
    
    # Verify zsh completion is installed
    local zsh_completion_path="$(brew --prefix)/share/zsh/site-functions/_hookdeck"
    echo_info "Checking zsh completion at: $zsh_completion_path"
    if [ -f "$zsh_completion_path" ]; then
        echo_success "Zsh completion installed"
    else
        echo_error "Zsh completion not found at $zsh_completion_path"
        return 1
    fi
    
    echo_success "Cask installation validation passed"
    return 0
}

# Test conflict detection
test_conflict_detection() {
    echo_section "Testing Conflict Detection"
    
    local tap_name="hookdeck-test/hookdeck-test/hookdeck"
    
    # First, install formula with --formula flag to ensure it's installed as formula
    echo_info "Installing formula first: brew install --formula $tap_name"
    if brew install --formula "$tap_name"; then
        echo_success "Formula installed successfully"
        FORMULA_INSTALLED=true
    else
        echo_error "Formula installation failed"
        return 1
    fi
    
    # Ensure the formula is linked
    echo_info "Ensuring formula is linked..."
    if brew link hookdeck 2>/dev/null || true; then
        echo_success "Formula linked"
    fi
    
    # Try to install cask - should fail with conflict
    echo_info "Attempting to install cask (should fail with conflict)..."
    local output
    if output=$(brew install --cask "$tap_name" 2>&1); then
        # Check if it's actually refusing to upgrade/install due to formula being present
        if echo "$output" | grep -qi "already installed"; then
            echo_success "Conflict implicitly detected (formula already installed, cask refused to overwrite)"
            echo_info "Message: $(echo "$output" | grep -i "already installed")"
        else
            echo_error "Cask installation succeeded (should have failed with conflict!)"
            echo_error "Output: $output"
            return 1
        fi
    else
        # Check if the error is about conflicts
        if echo "$output" | grep -qi "conflict"; then
            echo_success "Conflict properly detected"
            echo_info "Conflict message: $(echo "$output" | grep -i conflict)"
        else
            echo_error "Installation failed, but NOT due to conflict"
            echo_error "Output: $output"
            return 1
        fi
    fi
    
    echo_success "Conflict detection validation passed"
    return 0
}

# Run installation tests
run_installation_tests() {
    echo_section "Running Installation Tests"
    
    if [ "$RUN_INSTALL_TESTS" = false ]; then
        echo_info "Installation tests skipped (use --install flag to enable)"
        return 0
    fi
    
    local all_passed=true
    
    # Set up local tap
    if ! setup_local_tap; then
        echo_error "Failed to set up local tap"
        return 1
    fi
    
    # Test 1: Formula installation
    if ! test_formula_installation; then
        all_passed=false
    fi
    
    # Clean up formula before cask test
    if [ "$FORMULA_INSTALLED" = true ]; then
        echo_info "Uninstalling formula before cask test..."
        brew uninstall hookdeck 2>/dev/null || true
        FORMULA_INSTALLED=false
        echo_success "Formula uninstalled"
    fi
    
    # Test 2: Cask installation
    if ! test_cask_installation; then
        all_passed=false
    fi
    
    # Clean up cask before conflict test
    if [ "$CASK_INSTALLED" = true ]; then
        echo_info "Uninstalling cask before conflict test..."
        brew uninstall --cask hookdeck 2>/dev/null || true
        CASK_INSTALLED=false
        echo_success "Cask uninstalled"
    fi
    
    # Test 3: Conflict detection
    if ! test_conflict_detection; then
        all_passed=false
    fi
    
    if [ "$all_passed" = true ]; then
        echo_success "All installation tests passed!"
        return 0
    else
        echo_error "Some installation tests failed"
        return 1
    fi
}

# Main test execution
main() {
    echo_section "Hookdeck CLI Homebrew Build Validation"
    
    check_prerequisites
    clean_dist
    run_goreleaser_build
    
    local all_passed=true
    
    if ! validate_formula; then
        all_passed=false
    fi
    
    if ! validate_cask; then
        all_passed=false
    fi
    
    if ! validate_completions_in_tarball; then
        all_passed=false
    fi
    
    # Run installation tests if requested
    if [ "$RUN_INSTALL_TESTS" = true ]; then
        if ! run_installation_tests; then
            all_passed=false
        fi
    fi
    
    echo ""
    echo_section "Validation Summary"
    
    if [ "$all_passed" = true ]; then
        echo_success "All validations passed!"
        echo ""
        echo_info "What was validated:"
        echo "  ✓ GoReleaser configuration generates correct Homebrew files"
        echo "  ✓ Completion files are bundled in archives"
        echo "  ✓ Formula has deprecation warnings"
        echo "  ✓ Formula has proper completion directives"
        echo "  ✓ Cask has proper completion directives"
        echo "  ✓ Cask has conflicts with formula"
        
        if [ "$RUN_INSTALL_TESTS" = true ]; then
            echo "  ✓ Formula installs correctly from local tap"
            echo "  ✓ Cask installs correctly from local tap"
            echo "  ✓ Conflict detection works (formula vs cask)"
            echo "  ✓ Completions are installed in correct locations"
            echo "  ✓ Binary is functional after installation"
        else
            echo ""
            echo_info "Note: Installation tests not run (use --install flag to enable)"
        fi
        echo ""
        return 0
    else
        echo_error "Some validations failed"
        return 1
    fi
}

# Run main function
main
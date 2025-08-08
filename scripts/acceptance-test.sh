#!/bin/bash

# Basic Acceptance Test for Hookdeck CLI
# --------------------------------------
# This script tests the following:
#   - Basic CLI functionality (build, version, help)
#   - Authentication with API key
#   - CI mode operation
#   - Listen command initialization
#
# Limitations in CI mode:
#   - Cannot test interactive workflows
#   - Source/destination creation and management not directly tested
#   - Connection creation not directly tested
#   - It seems that the CI mode is restricted to a single org and project
#     Therefore, switching between projects or orgs is not tested

set -e

if [ -z "$HOOKDECK_CLI_TESTING_API_KEY" ]; then
  echo "Error: HOOKDECK_CLI_TESTING_API_KEY environment variable is not set."
  exit 1
fi

# Add a function to echo commands before executing them
echo_and_run() {
    echo "Running command: $@"
    "$@"
}

echo "Running tests..."
echo_and_run go test ./...

echo "Building CLI..."
echo_and_run go build .

echo "Authenticating with API key..."
# Define CLI command variable (can be overridden from outside)
CLI_CMD=${CLI_CMD:-"./hookdeck-cli"}

echo "Checking CLI version..."
echo_and_run $CLI_CMD version

echo "Displaying CLI help..."
echo_and_run $CLI_CMD help

# Use the variable instead of hardcoded path
$CLI_CMD ci --api-key $HOOKDECK_CLI_TESTING_API_KEY

echo "Verifying authentication..."
echo_and_run $CLI_CMD whoami

echo "Testing listen command..."
echo_and_run $CLI_CMD listen 8080 "test-$(date +%Y%m%d%H%M%S)" &
PID=$!

# Wait for the listen command to initialize
echo "Waiting for 5 seconds to allow listen command to initialize..."
sleep 5

# Check if the process is still running
if ! kill -0 $PID 2>/dev/null; then
  echo "Error: The listen command failed to start properly"
  exit 1
fi

echo "Listen command successfully started with PID $PID"

kill $PID

echo "Calling logout..."
$CLI_CMD logout

echo "All tests passed!"
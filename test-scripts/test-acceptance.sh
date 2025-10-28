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

set -ex

# Load environment variables from .env file if it exists
if [ -f "test-scripts/.env" ]; then
    echo "Loading environment variables from test-scripts/.env"
    set -o allexport
    source "test-scripts/.env"
    set +o allexport
fi

if [ -z "$HOOKDECK_CLI_TESTING_API_KEY" ]; then
  echo "Error: HOOKDECK_CLI_TESTING_API_KEY environment variable is not set."
  exit 1
fi

# Add a function to echo commands before executing them
echo_and_run() {
    echo "Running command: $@"
    "$@"
}

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
# Redirect stdin from /dev/null to signal non-interactive mode
# This will auto-create the source without prompting
echo_and_run $CLI_CMD listen 8080 "test-$(date +%Y%m%d%H%M%S)" --output compact < /dev/null &
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

kill -TERM $PID

echo "Testing connection commands..."

# Test connection list
echo "Listing connections..."
echo_and_run $CLI_CMD connection list

# Test connection create with various inline source authentications
# We will store the names of the created connections to delete them later
declare -a CREATED_CONNECTION_IDS

create_and_track_connection() {
    local conn_name=$1
    shift
    echo "Creating connection '$conn_name'..."
    # Add --output json and parse the ID from the output using jq
    # The actual command output is captured, while echo_and_run still prints the command
    echo "Running command: $CLI_CMD connection create --name \"$conn_name\" --output json $@"
    output=$($CLI_CMD connection create --name "$conn_name" --output json "$@")
    conn_id=$(echo "$output" | jq -r '.id')

    if [ -z "$conn_id" ] || [ "$conn_id" == "null" ]; then
        echo "Error: Failed to create connection or parse its ID."
        echo "Output: $output"
        exit 1
    fi

    CREATED_CONNECTION_IDS+=("$conn_id")
    echo "Successfully created connection with ID: $conn_id"
    echo "---"
}

# --- Test Case 1: Simple WEBHOOK source (no auth) ---
CONN_NAME_WEBHOOK="test-conn-webhook-$(date +%Y%m%d%H%M%S)"
create_and_track_connection "$CONN_NAME_WEBHOOK" \
  --source-name "test-src-webhook-$(date +%Y%m%d%H%M%S)" \
  --source-type "WEBHOOK" \
  --destination-name "test-dst-cli-$(date +%Y%m%d%H%M%S)" \
  --destination-type "CLI" \
  --destination-cli-path "/webhooks"

# --- Test Case 2: STRIPE source (webhook secret auth) ---
CONN_NAME_STRIPE="test-conn-stripe-$(date +%Y%m%d%H%M%S)"
create_and_track_connection "$CONN_NAME_STRIPE" \
  --source-name "test-src-stripe-$(date +%Y%m%d%H%M%S)" \
  --source-type "STRIPE" \
  --source-webhook-secret "whsec_testsecret123" \
  --destination-name "test-dst-cli-stripe-$(date +%Y%m%d%H%M%S)" \
  --destination-type "CLI" \
  --destination-cli-path "/webhooks"

# --- Test Case 3: GENERIC source (API key auth) ---
# Using a generic type that supports api_key
CONN_NAME_APIKEY="test-conn-apikey-$(date +%Y%m%d%H%M%S)"
create_and_track_connection "$CONN_NAME_APIKEY" \
  --source-name "test-src-apikey-$(date +%Y%m%d%H%M%S)" \
  --source-type "GENERIC" \
  --source-api-key "test-api-key-123" \
  --destination-name "test-dst-cli-apikey-$(date +%Y%m%d%H%M%S)" \
  --destination-type "CLI" \
  --destination-cli-path "/webhooks"

# --- Test Case 4: HTTP source (basic auth) ---
CONN_NAME_BASICAUTH="test-conn-basicauth-$(date +%Y%m%d%H%M%S)"
create_and_track_connection "$CONN_NAME_BASICAUTH" \
  --source-name "test-src-basicauth-$(date +%Y%m%d%H%M%S)" \
  --source-type "HTTP" \
  --source-basic-auth-user "testuser" \
  --source-basic-auth-pass "testpass" \
  --destination-name "test-dst-cli-basicauth-$(date +%Y%m%d%H%M%S)" \
  --destination-type "CLI" \
  --destination-cli-path "/webhooks"

# --- Test Case 5: TWILIO source (HMAC auth) ---
CONN_NAME_HMAC="test-conn-hmac-$(date +%Y%m%d%H%M%S)"
create_and_track_connection "$CONN_NAME_HMAC" \
  --source-name "test-src-hmac-$(date +%Ym%d%H%M%S)" \
  --source-type "TWILIO" \
  --source-hmac-secret "test-hmac-secret" \
  --source-hmac-algo "sha1" \
  --destination-name "test-dst-cli-hmac-$(date +%Y%m%d%H%M%S)" \
  --destination-type "CLI" \
  --destination-cli-path "/webhooks"

echo "All connection creation tests passed."

# --- Test Case 6: Connection Update ---
echo "Testing connection update..."
CONN_NAME_UPDATE="test-conn-update-$(date +%Y%m%d%H%M%S)"
create_and_track_connection "$CONN_NAME_UPDATE" \
  --source-name "test-src-update-$(date +%Y%m%d%H%M%S)" \
  --source-type "WEBHOOK" \
  --destination-name "test-dst-update-$(date +%Y%m%d%H%M%S)" \
  --destination-type "CLI" \
  --destination-cli-path "/webhooks"

# The ID of the connection to update is the last one added to the array
# Using a more compatible way to get the last element's index
last_index=$((${#CREATED_CONNECTION_IDS[@]} - 1))
UPDATE_CONN_ID=${CREATED_CONNECTION_IDS[$last_index]}
NEW_NAME="updated-conn-name-$(date +%Y%m%d%H%M%S)"
NEW_DESC="This is an updated description."

echo "Updating connection ID: $UPDATE_CONN_ID"
echo_and_run $CLI_CMD connection update "$UPDATE_CONN_ID" --name "$NEW_NAME" --description "$NEW_DESC"

echo "Verifying update..."
echo "Running command: $CLI_CMD connection get \"$UPDATE_CONN_ID\" --output json"
updated_conn_json=$($CLI_CMD connection get "$UPDATE_CONN_ID" --output json)
updated_name=$(echo "$updated_conn_json" | jq -r '.name')
updated_desc=$(echo "$updated_conn_json" | jq -r '.description')

if [ "$updated_name" != "$NEW_NAME" ]; then
    echo "Error: Connection name was not updated correctly."
    echo "Expected: $NEW_NAME, Got: $updated_name"
    exit 1
fi

if [ "$updated_desc" != "$NEW_DESC" ]; then
    echo "Error: Connection description was not updated correctly."
    echo "Expected: $NEW_DESC, Got: $updated_desc"
    exit 1
fi

echo "Connection update tested successfully."

# --- Cleanup ---
echo "Cleaning up created connections..."
for conn_id in "${CREATED_CONNECTION_IDS[@]}"; do
    echo "Deleting connection ID: $conn_id"
    # Use --force to bypass interactive prompt in CI
    echo_and_run $CLI_CMD connection delete "$conn_id" --force
done

echo "Cleanup complete."
echo "Connection commands tested successfully"

echo "Calling logout..."
$CLI_CMD logout

echo "All tests passed!"

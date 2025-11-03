#!/bin/bash

# This script deletes all connections in the currently configured project.

set -e

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

CLI_CMD=${CLI_CMD:-"./hookdeck-cli"}

# Authenticate in CI mode
$CLI_CMD ci --api-key $HOOKDECK_CLI_TESTING_API_KEY

echo "Fetching all connection IDs..."
# Get all connections in JSON format and extract just the IDs
CONNECTION_IDS=$($CLI_CMD connection list --output json | jq -r '.[].id')

if [ -z "$CONNECTION_IDS" ]; then
    echo "No connections found to delete."
    exit 0
fi

echo "Found connections to delete:"
echo "$CONNECTION_IDS"
echo "---"

# Loop through and delete each connection
# Confirm with the user before deleting
echo "You are about to delete all connections in this project."
read -p "Are you sure you want to continue? [y/N]: " response
if [[ "$response" != "y" ]] && [[ "$response" != "Y" ]]; then
    echo "Deletion cancelled."
    exit 0
fi

for conn_id in $CONNECTION_IDS; do
    echo "Deleting connection ID: $conn_id"
    $CLI_CMD connection delete "$conn_id" --force
done

echo "---"
echo "All connections have been deleted."
#!/bin/bash

# Test script to verify Hookdeck API upsert behavior
# Tests whether source/destination are required when updating a connection

set -e

echo "Testing API behavior for connection upsert..."

# Get API key from test env file
HOOKDECK_API_KEY="2pa5f5oeqbcgj91tipwlob0n5h7bg1ptd1nxodx5wgw05b51s8"

# Generate unique name
CONN_NAME="test-api-behavior-$(date +%s)"

echo ""
echo "=== Step 1: Creating connection with source and destination ==="
CREATE_RESPONSE=$(curl -s -X PUT "https://api.hookdeck.com/2025-07-01/connections" \
  -H "Authorization: Bearer $HOOKDECK_API_KEY" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"$CONN_NAME\",
    \"description\": \"Initial description\",
    \"source\": {
      \"name\": \"test-source-$CONN_NAME\",
      \"type\": \"WEBHOOK\"
    },
    \"destination\": {
      \"name\": \"test-dest-$CONN_NAME\",
      \"type\": \"MOCK_API\"
    }
  }")

echo "$CREATE_RESPONSE" | jq -r '{id: .id, name: .name, description: .description, source: .source.name, destination: .destination.name}'

CONN_ID=$(echo "$CREATE_RESPONSE" | jq -r '.id')

echo ""
echo "=== Step 2: Updating ONLY description (no source/destination in request) ==="
UPDATE_RESPONSE=$(curl -s -X PUT "https://api.hookdeck.com/2025-07-01/connections" \
  -H "Authorization: Bearer $HOOKDECK_API_KEY" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"$CONN_NAME\",
    \"description\": \"Updated description WITHOUT source/destination\"
  }")

echo ""
echo "Response:"
echo "$UPDATE_RESPONSE" | jq '.'

echo ""
echo "=== Step 3: Cleanup ==="
curl -s -X DELETE "https://api.hookdeck.com/2025-07-01/connections/$CONN_ID" \
  -H "Authorization: Bearer $HOOKDECK_API_KEY" > /dev/null

echo "Deleted connection $CONN_ID"
echo ""
echo "Test complete!"
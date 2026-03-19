//go:build connection

package acceptance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConnectionCreateWithRulesJSONExactValues verifies that connection --rules (JSON string)
// sends the correct structure to the API and the returned rules preserve exact values.
func TestConnectionCreateWithRulesJSONExactValues(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	t.Run("filter rule with nested JSON values", func(t *testing.T) {
		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-rules-json-filter-" + timestamp
		sourceName := "test-rules-json-src-" + timestamp
		destName := "test-rules-json-dst-" + timestamp

		rulesJSON := `[{"type":"filter","headers":{"x-event-type":{"$eq":"order.created"}},"body":{"amount":{"$gte":100},"currency":"USD"}}]`

		var resp map[string]interface{}
		err := cli.RunJSON(&resp,
			"gateway", "connection", "create",
			"--name", connName,
			"--source-type", "WEBHOOK",
			"--source-name", sourceName,
			"--destination-type", "HTTP",
			"--destination-name", destName,
			"--destination-url", "https://example.com/webhook",
			"--rules", rulesJSON,
		)
		require.NoError(t, err, "Should create connection with --rules JSON")

		connID, ok := resp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID")
		t.Cleanup(func() { deleteConnection(t, cli, connID) })

		rules, ok := resp["rules"].([]interface{})
		require.True(t, ok, "Expected rules array")

		foundFilter := false
		for _, r := range rules {
			rule, ok := r.(map[string]interface{})
			if !ok || rule["type"] != "filter" {
				continue
			}
			foundFilter = true

			// Verify headers with exact nested values
			headersMap, ok := rule["headers"].(map[string]interface{})
			require.True(t, ok, "headers should be a map")
			eventTypeMap, ok := headersMap["x-event-type"].(map[string]interface{})
			require.True(t, ok, "x-event-type should be a nested map")
			assert.Equal(t, "order.created", eventTypeMap["$eq"],
				"$eq value should be exactly 'order.created'")

			// Verify body with exact nested values
			bodyMap, ok := rule["body"].(map[string]interface{})
			require.True(t, ok, "body should be a map")
			assert.Equal(t, "USD", bodyMap["currency"],
				"currency should be exactly 'USD'")
			amountMap, ok := bodyMap["amount"].(map[string]interface{})
			require.True(t, ok, "amount should be a nested map")
			assert.Equal(t, float64(100), amountMap["$gte"],
				"$gte should be exactly 100")
			break
		}
		assert.True(t, foundFilter, "Should have a filter rule")
	})

	t.Run("multiple rules with exact values", func(t *testing.T) {
		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-rules-json-multi-" + timestamp
		sourceName := "test-rules-json-multi-src-" + timestamp
		destName := "test-rules-json-multi-dst-" + timestamp

		rulesJSON := `[{"type":"filter","headers":{"content-type":"application/json"}},{"type":"retry","strategy":"linear","count":3,"interval":10000,"response_status_codes":[500,502,503]}]`

		var resp map[string]interface{}
		err := cli.RunJSON(&resp,
			"gateway", "connection", "create",
			"--name", connName,
			"--source-type", "WEBHOOK",
			"--source-name", sourceName,
			"--destination-type", "HTTP",
			"--destination-name", destName,
			"--destination-url", "https://example.com/webhook",
			"--rules", rulesJSON,
		)
		require.NoError(t, err, "Should create connection with multiple --rules")

		connID, ok := resp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID")
		t.Cleanup(func() { deleteConnection(t, cli, connID) })

		rules, ok := resp["rules"].([]interface{})
		require.True(t, ok, "Expected rules array")

		foundFilter := false
		foundRetry := false
		for _, r := range rules {
			rule, ok := r.(map[string]interface{})
			if !ok {
				continue
			}

			switch rule["type"] {
			case "filter":
				foundFilter = true
				headersMap, ok := rule["headers"].(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "application/json", headersMap["content-type"])

			case "retry":
				foundRetry = true
				assert.Equal(t, "linear", rule["strategy"])
				assert.Equal(t, float64(3), rule["count"])
				assert.Equal(t, float64(10000), rule["interval"])

				statusCodes, ok := rule["response_status_codes"]
				require.True(t, ok, "response_status_codes should be present")
				assertResponseStatusCodesMatch(t, statusCodes, "500", "502", "503")
			}
		}
		assert.True(t, foundFilter, "Should have a filter rule")
		assert.True(t, foundRetry, "Should have a retry rule")
	})
}

// TestConnectionCreateWithRulesFileExactValues verifies that connection --rules-file
// reads JSON from a file and the returned rules preserve exact values.
func TestConnectionCreateWithRulesFileExactValues(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	connName := "test-rules-file-" + timestamp
	sourceName := "test-rules-file-src-" + timestamp
	destName := "test-rules-file-dst-" + timestamp

	rulesContent := `[{"type":"filter","headers":{"x-source":{"$eq":"stripe"}},"body":{"event":{"$startsWith":"payment."}}},{"type":"retry","strategy":"exponential","count":5,"interval":30000,"response_status_codes":[500,502,503,504]}]`
	tmpFile := filepath.Join(t.TempDir(), "rules.json")
	require.NoError(t, os.WriteFile(tmpFile, []byte(rulesContent), 0644))

	var resp map[string]interface{}
	err := cli.RunJSON(&resp,
		"gateway", "connection", "create",
		"--name", connName,
		"--source-type", "WEBHOOK",
		"--source-name", sourceName,
		"--destination-type", "HTTP",
		"--destination-name", destName,
		"--destination-url", "https://example.com/webhook",
		"--rules-file", tmpFile,
	)
	require.NoError(t, err, "Should create connection with --rules-file")

	connID, ok := resp["id"].(string)
	require.True(t, ok && connID != "", "Expected connection ID")
	t.Cleanup(func() { deleteConnection(t, cli, connID) })

	rules, ok := resp["rules"].([]interface{})
	require.True(t, ok, "Expected rules array")

	foundFilter := false
	foundRetry := false
	for _, r := range rules {
		rule, ok := r.(map[string]interface{})
		if !ok {
			continue
		}

		switch rule["type"] {
		case "filter":
			foundFilter = true

			headersMap, ok := rule["headers"].(map[string]interface{})
			require.True(t, ok)
			sourceMap, ok := headersMap["x-source"].(map[string]interface{})
			require.True(t, ok)
			assert.Equal(t, "stripe", sourceMap["$eq"])

			bodyMap, ok := rule["body"].(map[string]interface{})
			require.True(t, ok)
			eventMap, ok := bodyMap["event"].(map[string]interface{})
			require.True(t, ok)
			assert.Equal(t, "payment.", eventMap["$startsWith"])

		case "retry":
			foundRetry = true
			assert.Equal(t, "exponential", rule["strategy"])
			assert.Equal(t, float64(5), rule["count"])
			assert.Equal(t, float64(30000), rule["interval"])

			statusCodes, ok := rule["response_status_codes"]
			require.True(t, ok, "response_status_codes should be present")
			assertResponseStatusCodesMatch(t, statusCodes, "500", "502", "503", "504")
		}
	}
	assert.True(t, foundFilter, "Should have a filter rule")
	assert.True(t, foundRetry, "Should have a retry rule")
}

// TestConnectionUpsertWithRulesJSONExactValues verifies that connection upsert --rules (JSON)
// sends the correct structure and the returned rules preserve exact values.
func TestConnectionUpsertWithRulesJSONExactValues(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	connName := "test-upsert-rules-json-" + timestamp
	sourceName := "test-upsert-rules-src-" + timestamp
	destName := "test-upsert-rules-dst-" + timestamp

	rulesJSON := `[{"type":"filter","body":{"action":{"$in":["created","updated"]}}},{"type":"delay","delay":5000}]`

	var resp map[string]interface{}
	err := cli.RunJSON(&resp,
		"gateway", "connection", "upsert", connName,
		"--source-type", "WEBHOOK",
		"--source-name", sourceName,
		"--destination-type", "HTTP",
		"--destination-name", destName,
		"--destination-url", "https://example.com/webhook",
		"--rules", rulesJSON,
	)
	require.NoError(t, err, "Should upsert connection with --rules JSON")

	connID, ok := resp["id"].(string)
	require.True(t, ok && connID != "", "Expected connection ID")
	t.Cleanup(func() { deleteConnection(t, cli, connID) })

	rules, ok := resp["rules"].([]interface{})
	require.True(t, ok, "Expected rules array")

	foundFilter := false
	foundDelay := false
	for _, r := range rules {
		rule, ok := r.(map[string]interface{})
		if !ok {
			continue
		}

		switch rule["type"] {
		case "filter":
			foundFilter = true
			bodyMap, ok := rule["body"].(map[string]interface{})
			require.True(t, ok)
			actionMap, ok := bodyMap["action"].(map[string]interface{})
			require.True(t, ok)
			inArr, ok := actionMap["$in"].([]interface{})
			require.True(t, ok, "$in should be an array")
			assert.Equal(t, []interface{}{"created", "updated"}, inArr)

		case "delay":
			foundDelay = true
			assert.Equal(t, float64(5000), rule["delay"], "delay should be exactly 5000")
		}
	}
	assert.True(t, foundFilter, "Should have a filter rule")
	assert.True(t, foundDelay, "Should have a delay rule")
}

package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildConnectionRulesFromJSONString verifies that --rules (JSON string) parses
// into a rules slice with exact values and structure preserved.
func TestBuildConnectionRulesFromJSONString(t *testing.T) {
	t.Run("filter rule JSON with exact nested values", func(t *testing.T) {
		input := `[{"type":"filter","headers":{"x-shopify-topic":{"$startsWith":"order/"}},"body":{"event_type":"payment"}}]`
		flags := connectionRuleFlags{Rules: input}
		rules, err := buildConnectionRules(&flags)
		require.NoError(t, err)
		require.Len(t, rules, 1)

		rule := rules[0]
		assert.Equal(t, "filter", rule["type"])

		headersMap, ok := rule["headers"].(map[string]interface{})
		require.True(t, ok, "headers should be a map, got %T", rule["headers"])

		topicMap, ok := headersMap["x-shopify-topic"].(map[string]interface{})
		require.True(t, ok, "x-shopify-topic should be a nested map")
		assert.Equal(t, "order/", topicMap["$startsWith"])

		bodyMap, ok := rule["body"].(map[string]interface{})
		require.True(t, ok, "body should be a map")
		assert.Equal(t, "payment", bodyMap["event_type"])
	})

	t.Run("retry rule JSON with exact numeric values", func(t *testing.T) {
		input := `[{"type":"retry","strategy":"exponential","count":5,"interval":30000,"response_status_codes":[500,502,503]}]`
		flags := connectionRuleFlags{Rules: input}
		rules, err := buildConnectionRules(&flags)
		require.NoError(t, err)
		require.Len(t, rules, 1)

		rule := rules[0]
		assert.Equal(t, "retry", rule["type"])
		assert.Equal(t, "exponential", rule["strategy"])
		assert.Equal(t, float64(5), rule["count"])
		assert.Equal(t, float64(30000), rule["interval"])

		statusCodes, ok := rule["response_status_codes"].([]string)
		require.True(t, ok, "response_status_codes should be []string (API schema), got %T", rule["response_status_codes"])
		assert.Equal(t, []string{"500", "502", "503"}, statusCodes)
	})

	t.Run("multiple rules JSON preserves all rules with exact values", func(t *testing.T) {
		input := `[{"type":"filter","headers":{"content-type":"application/json"}},{"type":"delay","delay":5000},{"type":"retry","strategy":"linear","count":3,"interval":10000}]`
		flags := connectionRuleFlags{Rules: input}
		rules, err := buildConnectionRules(&flags)
		require.NoError(t, err)
		require.Len(t, rules, 3)

		// Filter rule
		assert.Equal(t, "filter", rules[0]["type"])
		headersMap, ok := rules[0]["headers"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "application/json", headersMap["content-type"])

		// Delay rule
		assert.Equal(t, "delay", rules[1]["type"])
		assert.Equal(t, float64(5000), rules[1]["delay"])

		// Retry rule
		assert.Equal(t, "retry", rules[2]["type"])
		assert.Equal(t, "linear", rules[2]["strategy"])
		assert.Equal(t, float64(3), rules[2]["count"])
		assert.Equal(t, float64(10000), rules[2]["interval"])
	})

	t.Run("transform rule JSON preserves transformation config", func(t *testing.T) {
		input := `[{"type":"transform","transformation":{"name":"my-transform","env":{"API_KEY":"sk-123","MODE":"production"}}}]`
		flags := connectionRuleFlags{Rules: input}
		rules, err := buildConnectionRules(&flags)
		require.NoError(t, err)
		require.Len(t, rules, 1)

		rule := rules[0]
		assert.Equal(t, "transform", rule["type"])

		transformation, ok := rule["transformation"].(map[string]interface{})
		require.True(t, ok, "transformation should be a map")
		assert.Equal(t, "my-transform", transformation["name"])

		env, ok := transformation["env"].(map[string]interface{})
		require.True(t, ok, "env should be a map")
		assert.Equal(t, "sk-123", env["API_KEY"])
		assert.Equal(t, "production", env["MODE"])
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		flags := connectionRuleFlags{Rules: `[{broken`}
		_, err := buildConnectionRules(&flags)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--rules")
	})
}

// TestBuildConnectionRulesFromJSONFile verifies that --rules-file reads a JSON file
// and produces rules with exact values and structure preserved.
func TestBuildConnectionRulesFromJSONFile(t *testing.T) {
	t.Run("file with filter and retry rules preserves exact values", func(t *testing.T) {
		content := `[{"type":"filter","headers":{"x-event-type":{"$eq":"order.created"}},"body":{"amount":{"$gte":100}}},{"type":"retry","strategy":"linear","count":3,"interval":5000,"response_status_codes":[500,502]}]`
		tmpFile := filepath.Join(t.TempDir(), "rules.json")
		require.NoError(t, os.WriteFile(tmpFile, []byte(content), 0644))

		flags := connectionRuleFlags{RulesFile: tmpFile}
		rules, err := buildConnectionRules(&flags)
		require.NoError(t, err)
		require.Len(t, rules, 2)

		// Filter rule with exact nested values
		filterRule := rules[0]
		assert.Equal(t, "filter", filterRule["type"])

		headersMap, ok := filterRule["headers"].(map[string]interface{})
		require.True(t, ok)
		eventTypeMap, ok := headersMap["x-event-type"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "order.created", eventTypeMap["$eq"])

		bodyMap, ok := filterRule["body"].(map[string]interface{})
		require.True(t, ok)
		amountMap, ok := bodyMap["amount"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, float64(100), amountMap["$gte"])

		// Retry rule with exact values
		retryRule := rules[1]
		assert.Equal(t, "retry", retryRule["type"])
		assert.Equal(t, "linear", retryRule["strategy"])
		assert.Equal(t, float64(3), retryRule["count"])
		assert.Equal(t, float64(5000), retryRule["interval"])

		statusCodes, ok := retryRule["response_status_codes"].([]string)
		require.True(t, ok, "response_status_codes should be []string (API schema)")
		assert.Equal(t, []string{"500", "502"}, statusCodes)
	})

	t.Run("file with deduplicate rule preserves fields", func(t *testing.T) {
		content := `[{"type":"deduplicate","window":3600,"include_fields":["id","timestamp"]}]`
		tmpFile := filepath.Join(t.TempDir(), "dedup-rules.json")
		require.NoError(t, os.WriteFile(tmpFile, []byte(content), 0644))

		flags := connectionRuleFlags{RulesFile: tmpFile}
		rules, err := buildConnectionRules(&flags)
		require.NoError(t, err)
		require.Len(t, rules, 1)

		rule := rules[0]
		assert.Equal(t, "deduplicate", rule["type"])
		assert.Equal(t, float64(3600), rule["window"])

		fields, ok := rule["include_fields"].([]interface{})
		require.True(t, ok, "include_fields should be an array")
		assert.Equal(t, []interface{}{"id", "timestamp"}, fields)
	})

	t.Run("nonexistent file returns error", func(t *testing.T) {
		flags := connectionRuleFlags{RulesFile: "/nonexistent/rules.json"}
		_, err := buildConnectionRules(&flags)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "rules file")
	})

	t.Run("invalid JSON file returns error", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "bad-rules.json")
		require.NoError(t, os.WriteFile(tmpFile, []byte(`[{invalid`), 0644))

		flags := connectionRuleFlags{RulesFile: tmpFile}
		_, err := buildConnectionRules(&flags)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "rules file")
	})
}

package cmd

import (
	"encoding/json"
	"testing"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strPtr(s string) *string {
	return &s
}

// TestBuildConnectionRulesFilterJSON verifies that all --rule-filter-* flags
// parse JSON values into objects with exact values preserved, not stored as
// escaped strings.
// Regression test for https://github.com/hookdeck/hookdeck-cli/issues/192.
func TestBuildConnectionRulesFilterJSON(t *testing.T) {
	t.Run("headers JSON parsed with exact nested values", func(t *testing.T) {
		flags := connectionRuleFlags{
			RuleFilterHeaders: `{"x-shopify-topic":{"$startsWith":"order/"}}`,
		}
		rules, err := buildConnectionRules(&flags)
		require.NoError(t, err)
		require.Len(t, rules, 1)

		filterRule := rules[0]
		assert.Equal(t, "filter", filterRule["type"])

		headersMap, ok := filterRule["headers"].(map[string]interface{})
		require.True(t, ok, "headers should be map[string]interface{}, got %T", filterRule["headers"])

		nestedMap, ok := headersMap["x-shopify-topic"].(map[string]interface{})
		require.True(t, ok, "x-shopify-topic should be a nested object, got %T", headersMap["x-shopify-topic"])
		assert.Equal(t, "order/", nestedMap["$startsWith"], "nested $startsWith value should match exactly")
	})

	t.Run("body JSON parsed with exact values", func(t *testing.T) {
		flags := connectionRuleFlags{
			RuleFilterBody: `{"event_type":"payment","amount":{"$gte":100}}`,
		}
		rules, err := buildConnectionRules(&flags)
		require.NoError(t, err)
		require.Len(t, rules, 1)

		bodyMap, ok := rules[0]["body"].(map[string]interface{})
		require.True(t, ok, "body should be map[string]interface{}, got %T", rules[0]["body"])
		assert.Equal(t, "payment", bodyMap["event_type"], "event_type value should match exactly")

		amountMap, ok := bodyMap["amount"].(map[string]interface{})
		require.True(t, ok, "amount should be a nested object, got %T", bodyMap["amount"])
		assert.Equal(t, float64(100), amountMap["$gte"], "$gte value should match exactly")
	})

	t.Run("query JSON parsed with exact values", func(t *testing.T) {
		flags := connectionRuleFlags{
			RuleFilterQuery: `{"status":"active","page":{"$gte":1}}`,
		}
		rules, err := buildConnectionRules(&flags)
		require.NoError(t, err)
		require.Len(t, rules, 1)

		queryMap, ok := rules[0]["query"].(map[string]interface{})
		require.True(t, ok, "query should be map[string]interface{}, got %T", rules[0]["query"])
		assert.Equal(t, "active", queryMap["status"], "status value should match exactly")

		pageMap, ok := queryMap["page"].(map[string]interface{})
		require.True(t, ok, "page should be a nested object, got %T", queryMap["page"])
		assert.Equal(t, float64(1), pageMap["$gte"], "$gte value should match exactly")
	})

	t.Run("path JSON parsed with exact values", func(t *testing.T) {
		flags := connectionRuleFlags{
			RuleFilterPath: `{"$contains":"/webhooks/"}`,
		}
		rules, err := buildConnectionRules(&flags)
		require.NoError(t, err)
		require.Len(t, rules, 1)

		pathMap, ok := rules[0]["path"].(map[string]interface{})
		require.True(t, ok, "path should be map[string]interface{}, got %T", rules[0]["path"])
		assert.Equal(t, "/webhooks/", pathMap["$contains"], "$contains value should match exactly")
	})

	t.Run("all four filter flags combined with exact values", func(t *testing.T) {
		flags := connectionRuleFlags{
			RuleFilterHeaders: `{"content-type":"application/json"}`,
			RuleFilterBody:    `{"action":"created"}`,
			RuleFilterQuery:   `{"verbose":"true"}`,
			RuleFilterPath:    `{"$startsWith":"/api/v1"}`,
		}
		rules, err := buildConnectionRules(&flags)
		require.NoError(t, err)
		require.Len(t, rules, 1)

		rule := rules[0]
		assert.Equal(t, "filter", rule["type"])

		headersMap, ok := rule["headers"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "application/json", headersMap["content-type"])

		bodyMap, ok := rule["body"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "created", bodyMap["action"])

		queryMap, ok := rule["query"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "true", queryMap["verbose"])

		pathMap, ok := rule["path"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "/api/v1", pathMap["$startsWith"])
	})

	t.Run("JSON round-trip preserves exact structure", func(t *testing.T) {
		input := `{"x-shopify-topic":{"$startsWith":"order/"},"x-api-key":{"$eq":"secret123"}}`
		flags := connectionRuleFlags{
			RuleFilterHeaders: input,
		}
		rules, err := buildConnectionRules(&flags)
		require.NoError(t, err)
		require.Len(t, rules, 1)

		// Marshal the rule to JSON and unmarshal back to verify round-trip
		jsonBytes, err := json.Marshal(rules[0])
		require.NoError(t, err)

		var parsed map[string]interface{}
		require.NoError(t, json.Unmarshal(jsonBytes, &parsed))

		headersMap, ok := parsed["headers"].(map[string]interface{})
		require.True(t, ok)

		topicMap, ok := headersMap["x-shopify-topic"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "order/", topicMap["$startsWith"])

		apiKeyMap, ok := headersMap["x-api-key"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "secret123", apiKeyMap["$eq"])
	})

	t.Run("JSON array values parsed correctly", func(t *testing.T) {
		flags := connectionRuleFlags{
			RuleFilterBody: `{"tags":["urgent","billing"],"status":{"$in":["active","pending"]}}`,
		}
		rules, err := buildConnectionRules(&flags)
		require.NoError(t, err)
		require.Len(t, rules, 1)

		bodyMap, ok := rules[0]["body"].(map[string]interface{})
		require.True(t, ok)

		tags, ok := bodyMap["tags"].([]interface{})
		require.True(t, ok, "tags should be an array, got %T", bodyMap["tags"])
		assert.Equal(t, []interface{}{"urgent", "billing"}, tags)

		statusMap, ok := bodyMap["status"].(map[string]interface{})
		require.True(t, ok)
		inArr, ok := statusMap["$in"].([]interface{})
		require.True(t, ok, "$in should be an array, got %T", statusMap["$in"])
		assert.Equal(t, []interface{}{"active", "pending"}, inArr)
	})

	t.Run("non-JSON string should remain a string", func(t *testing.T) {
		flags := connectionRuleFlags{
			RuleFilterHeaders: `.["x-topic"] == "order"`,
		}
		rules, err := buildConnectionRules(&flags)
		require.NoError(t, err)
		require.Len(t, rules, 1)

		headers := rules[0]["headers"]
		_, isString := headers.(string)
		assert.True(t, isString, "non-JSON value should remain a string")
		assert.Equal(t, `.["x-topic"] == "order"`, headers)
	})

	t.Run("bare JSON primitives should remain as strings", func(t *testing.T) {
		for _, input := range []string{`"order"`, `123`, `true`} {
			flags := connectionRuleFlags{
				RuleFilterHeaders: input,
			}
			rules, err := buildConnectionRules(&flags)
			require.NoError(t, err)
			require.Len(t, rules, 1)

			headers := rules[0]["headers"]
			_, isString := headers.(string)
			assert.True(t, isString, "input %q should remain a string, got %T", input, headers)
			assert.Equal(t, input, headers, "value should be unchanged")
		}
	})
}

// TestBuildConnectionRulesTransformEnvJSON verifies that --rule-transform-env
// parses JSON values into objects with exact values preserved.
func TestBuildConnectionRulesTransformEnvJSON(t *testing.T) {
	t.Run("env JSON parsed with exact values", func(t *testing.T) {
		flags := connectionRuleFlags{
			RuleTransformName: "my-transform",
			RuleTransformEnv:  `{"API_KEY":"sk-test-123","DEBUG":"true","TIMEOUT":"30"}`,
		}
		rules, err := buildConnectionRules(&flags)
		require.NoError(t, err)
		require.Len(t, rules, 1)

		rule := rules[0]
		assert.Equal(t, "transform", rule["type"])

		transformation, ok := rule["transformation"].(map[string]interface{})
		require.True(t, ok, "transformation should be a map")
		assert.Equal(t, "my-transform", transformation["name"])

		env, ok := transformation["env"].(map[string]interface{})
		require.True(t, ok, "env should be a map, got %T", transformation["env"])
		assert.Equal(t, "sk-test-123", env["API_KEY"], "API_KEY should match exactly")
		assert.Equal(t, "true", env["DEBUG"], "DEBUG should match exactly")
		assert.Equal(t, "30", env["TIMEOUT"], "TIMEOUT should match exactly")
	})

	t.Run("env JSON round-trip preserves exact values", func(t *testing.T) {
		flags := connectionRuleFlags{
			RuleTransformName: "my-transform",
			RuleTransformEnv:  `{"SECRET":"abc123","NESTED":{"key":"val"}}`,
		}
		rules, err := buildConnectionRules(&flags)
		require.NoError(t, err)

		jsonBytes, err := json.Marshal(rules[0])
		require.NoError(t, err)

		var parsed map[string]interface{}
		require.NoError(t, json.Unmarshal(jsonBytes, &parsed))

		transformation := parsed["transformation"].(map[string]interface{})
		env := transformation["env"].(map[string]interface{})
		assert.Equal(t, "abc123", env["SECRET"])

		nested, ok := env["NESTED"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "val", nested["key"])
	})
}

// TestBuildConnectionRulesRetryStatusCodesArray verifies that buildConnectionRules
// produces response_status_codes as a []string array (API RetryRule schema).
// Regression test for https://github.com/hookdeck/hookdeck-cli/issues/209 Bug 3.
func TestBuildConnectionRulesRetryStatusCodesArray(t *testing.T) {
	tests := []struct {
		name          string
		flags         connectionRuleFlags
		wantCodes     []string
		wantCodeCount int
		wantRuleCount int
	}{
		{
			name: "comma-separated status codes should produce array",
			flags: connectionRuleFlags{
				RuleRetryStrategy:           "linear",
				RuleRetryCount:              3,
				RuleRetryInterval:           5000,
				RuleRetryResponseStatusCode: "500,502,503,504",
			},
			wantCodes:     []string{"500", "502", "503", "504"},
			wantCodeCount: 4,
			wantRuleCount: 1,
		},
		{
			name: "single status code should produce single-element array",
			flags: connectionRuleFlags{
				RuleRetryStrategy:           "exponential",
				RuleRetryResponseStatusCode: "500",
			},
			wantCodes:     []string{"500"},
			wantCodeCount: 1,
			wantRuleCount: 1,
		},
		{
			name: "status codes with spaces should be trimmed",
			flags: connectionRuleFlags{
				RuleRetryStrategy:           "linear",
				RuleRetryResponseStatusCode: "500, 502, 503",
			},
			wantCodes:     []string{"500", "502", "503"},
			wantCodeCount: 3,
			wantRuleCount: 1,
		},
		{
			name: "no status codes should not include response_status_codes",
			flags: connectionRuleFlags{
				RuleRetryStrategy: "linear",
				RuleRetryCount:    3,
			},
			wantCodes:     nil,
			wantCodeCount: 0,
			wantRuleCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules, err := buildConnectionRules(&tt.flags)
			require.NoError(t, err, "buildConnectionRules should not error")
			require.Len(t, rules, tt.wantRuleCount, "Expected %d rule(s)", tt.wantRuleCount)

			if tt.wantRuleCount == 0 {
				return
			}

			retryRule := rules[len(rules)-1]
			assert.Equal(t, "retry", retryRule["type"], "Last rule should be retry")

			if tt.wantCodes == nil {
				_, exists := retryRule["response_status_codes"]
				assert.False(t, exists, "response_status_codes should not be present when not specified")
				return
			}

			statusCodes, ok := retryRule["response_status_codes"]
			require.True(t, ok, "response_status_codes should be present")

			codesSlice, ok := statusCodes.([]string)
			require.True(t, ok, "response_status_codes should be []string (API schema), got %T", statusCodes)
			assert.Equal(t, tt.wantCodeCount, len(codesSlice))
			assert.Equal(t, tt.wantCodes, codesSlice)

			// Verify it serializes to a JSON array of strings
			jsonBytes, err := json.Marshal(retryRule)
			require.NoError(t, err)

			var parsed map[string]interface{}
			err = json.Unmarshal(jsonBytes, &parsed)
			require.NoError(t, err)

			jsonCodes, ok := parsed["response_status_codes"].([]interface{})
			require.True(t, ok, "JSON response_status_codes should be an array, got %T", parsed["response_status_codes"])
			assert.Len(t, jsonCodes, tt.wantCodeCount)
		})
	}
}

// TestUpsertBuildRequestRulesOnlyPreservesDestinationByID verifies that when
// upserting with only rule flags, the request uses destination_id (not a full
// destination object that could include incomplete auth config).
// Regression test for https://github.com/hookdeck/hookdeck-cli/issues/209 Bug 1.
func TestUpsertBuildRequestRulesOnlyPreservesDestinationByID(t *testing.T) {
	cu := &connectionUpsertCmd{
		connectionCreateCmd: &connectionCreateCmd{},
	}
	cu.name = "test-conn"
	// Only set rule flags, no source/destination flags
	cu.RuleRetryStrategy = "linear"
	cu.RuleRetryCount = 3

	existing := &hookdeck.Connection{
		ID:   "conn_123",
		Name: strPtr("test-conn"),
		Source: &hookdeck.Source{
			ID:   "src_123",
			Name: "test-source",
			Type: "WEBHOOK",
		},
		Destination: &hookdeck.Destination{
			ID:   "dst_123",
			Name: "test-dest",
			Type: "HTTP",
			Config: map[string]interface{}{
				"url":       "https://api.example.com",
				"auth_type": "AWS_SIGNATURE",
			},
		},
	}

	req, err := cu.buildUpsertRequest(existing, true)
	require.NoError(t, err)

	// Should reference existing destination by ID, not recreate it
	assert.NotNil(t, req.DestinationID, "Should use DestinationID")
	assert.Equal(t, "dst_123", *req.DestinationID)
	assert.Nil(t, req.Destination, "Should NOT send full Destination object")

	// Source should also be preserved by ID
	assert.NotNil(t, req.SourceID, "Should use SourceID")
	assert.Equal(t, "src_123", *req.SourceID)
	assert.Nil(t, req.Source, "Should NOT send full Source object")

	// Rules should be present
	assert.NotEmpty(t, req.Rules)
}

// TestUpsertHasAnyDestinationFlagIgnoresDefault verifies that hasAnyDestinationFlag
// returns false when no destination flags are explicitly set (the cli-path default
// of "/" was previously causing this to always return true).
// Regression test for https://github.com/hookdeck/hookdeck-cli/issues/209 Bug 1.
func TestUpsertHasAnyDestinationFlagIgnoresDefault(t *testing.T) {
	cu := &connectionUpsertCmd{
		connectionCreateCmd: &connectionCreateCmd{},
	}
	// No destination flags set at all (cli-path default is "" for upsert)

	result := cu.hasAnyDestinationFlag()
	assert.False(t, result, "hasAnyDestinationFlag should be false with no destination flags set")
}

// TestUpsertValidateSourceFlagsAllowsNameOnly verifies that validateSourceFlags
// allows --source-name without --source-type for the upsert command.
// Regression test for https://github.com/hookdeck/hookdeck-cli/issues/209 Bug 2.
func TestUpsertValidateSourceFlagsAllowsNameOnly(t *testing.T) {
	cu := &connectionUpsertCmd{
		connectionCreateCmd: &connectionCreateCmd{},
	}
	cu.sourceName = "my-source"
	// sourceType intentionally empty

	err := cu.validateSourceFlags()
	assert.NoError(t, err, "validateSourceFlags should allow --source-name alone for upsert")
}

// TestUpsertValidateDestinationFlagsAllowsNameOnly verifies the same relaxation
// for destination flags.
func TestUpsertValidateDestinationFlagsAllowsNameOnly(t *testing.T) {
	cu := &connectionUpsertCmd{
		connectionCreateCmd: &connectionCreateCmd{},
	}
	cu.destinationName = "my-dest"
	// destinationType intentionally empty

	err := cu.validateDestinationFlags()
	assert.NoError(t, err, "validateDestinationFlags should allow --destination-name alone for upsert")
}

// TestUpsertBuildRequestFillsSourceTypeFromExisting verifies that when
// --source-name is provided without --source-type during an update,
// the existing source type is used.
func TestUpsertBuildRequestFillsSourceTypeFromExisting(t *testing.T) {
	cu := &connectionUpsertCmd{
		connectionCreateCmd: &connectionCreateCmd{},
	}
	cu.name = "test-conn"
	cu.sourceName = "new-source-name"
	// sourceType intentionally empty - should be filled from existing

	existing := &hookdeck.Connection{
		ID:   "conn_123",
		Name: strPtr("test-conn"),
		Source: &hookdeck.Source{
			ID:   "src_123",
			Name: "old-source-name",
			Type: "WEBHOOK",
		},
		Destination: &hookdeck.Destination{
			ID:   "dst_123",
			Name: "test-dest",
			Type: "HTTP",
			Config: map[string]interface{}{
				"url": "https://api.example.com",
			},
		},
	}

	req, err := cu.buildUpsertRequest(existing, true)
	require.NoError(t, err)

	// Source should be an inline input (not just ID) since name was changed
	require.NotNil(t, req.Source, "Should have Source input for name change")
	assert.Equal(t, "new-source-name", req.Source.Name)
	assert.Equal(t, "WEBHOOK", req.Source.Type, "Should fill type from existing source")
}

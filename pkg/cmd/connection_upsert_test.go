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

// TestBuildConnectionRulesFilterHeadersJSON verifies that --rule-filter-headers
// parses JSON values into objects rather than storing them as escaped strings.
// Regression test for https://github.com/hookdeck/hookdeck-cli/issues/192.
func TestBuildConnectionRulesFilterHeadersJSON(t *testing.T) {
	t.Run("JSON object should be parsed, not stored as string", func(t *testing.T) {
		flags := connectionRuleFlags{
			RuleFilterHeaders: `{"x-shopify-topic":{"$startsWith":"order/"}}`,
		}
		rules, err := buildConnectionRules(&flags)
		require.NoError(t, err)
		require.Len(t, rules, 1)

		filterRule := rules[0]
		assert.Equal(t, "filter", filterRule["type"])

		headers := filterRule["headers"]
		_, isString := headers.(string)
		assert.False(t, isString, "headers should be a parsed object, not a string")

		headersMap, isMap := headers.(map[string]interface{})
		require.True(t, isMap, "headers should be map[string]interface{}, got %T", headers)
		assert.Contains(t, headersMap, "x-shopify-topic")
	})

	t.Run("non-JSON string should remain a string", func(t *testing.T) {
		flags := connectionRuleFlags{
			RuleFilterHeaders: `.["x-topic"] == "order"`,
		}
		rules, err := buildConnectionRules(&flags)
		require.NoError(t, err)
		require.Len(t, rules, 1)

		filterRule := rules[0]
		headers := filterRule["headers"]
		_, isString := headers.(string)
		assert.True(t, isString, "non-JSON value should remain a string")
	})

	t.Run("bare JSON primitives should remain as strings", func(t *testing.T) {
		// Values like "order", 123, true are valid JSON primitives but should
		// NOT be parsed — they should be passed through as plain strings.
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

	t.Run("filter body JSON should also be parsed", func(t *testing.T) {
		flags := connectionRuleFlags{
			RuleFilterBody: `{"event_type":"payment"}`,
		}
		rules, err := buildConnectionRules(&flags)
		require.NoError(t, err)
		require.Len(t, rules, 1)

		filterRule := rules[0]
		body := filterRule["body"]
		_, isString := body.(string)
		assert.False(t, isString, "body should be a parsed object, not a string")
		_, isMap := body.(map[string]interface{})
		require.True(t, isMap, "body should be map[string]interface{}, got %T", body)
	})
}

// TestBuildConnectionRulesRetryStatusCodesArray verifies that buildConnectionRules
// produces response_status_codes as a []int array (HTTP status codes are integers).
// Regression test for https://github.com/hookdeck/hookdeck-cli/issues/209 Bug 3.
func TestBuildConnectionRulesRetryStatusCodesArray(t *testing.T) {
	tests := []struct {
		name          string
		flags         connectionRuleFlags
		wantCodes     []int
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
			wantCodes:     []int{500, 502, 503, 504},
			wantCodeCount: 4,
			wantRuleCount: 1,
		},
		{
			name: "single status code should produce single-element array",
			flags: connectionRuleFlags{
				RuleRetryStrategy:           "exponential",
				RuleRetryResponseStatusCode: "500",
			},
			wantCodes:     []int{500},
			wantCodeCount: 1,
			wantRuleCount: 1,
		},
		{
			name: "status codes with spaces should be trimmed",
			flags: connectionRuleFlags{
				RuleRetryStrategy:           "linear",
				RuleRetryResponseStatusCode: "500, 502, 503",
			},
			wantCodes:     []int{500, 502, 503},
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

			codesSlice, ok := statusCodes.([]int)
			require.True(t, ok, "response_status_codes should be []int, got %T", statusCodes)
			assert.Equal(t, tt.wantCodeCount, len(codesSlice))
			assert.Equal(t, tt.wantCodes, codesSlice)

			// Verify it serializes to a JSON array of numbers
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

package cmd

import (
	"encoding/json"
	"testing"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBug3_BuildConnectionRulesStatusCodesArray tests that buildConnectionRules
// produces response_status_codes as a []string array, not a single string.
//
// Reproduces: https://github.com/hookdeck/hookdeck-cli/issues/209 Bug 3
func TestBug3_BuildConnectionRulesStatusCodesArray(t *testing.T) {
	tests := []struct {
		name           string
		flags          connectionRuleFlags
		wantCodes      []string
		wantCodeCount  int
		wantRuleCount  int
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

			retryRule := rules[len(rules)-1] // retry rule is always last
			assert.Equal(t, "retry", retryRule["type"], "Last rule should be retry")

			if tt.wantCodes == nil {
				_, exists := retryRule["response_status_codes"]
				assert.False(t, exists, "response_status_codes should not be present when not specified")
				return
			}

			statusCodes, ok := retryRule["response_status_codes"]
			require.True(t, ok, "response_status_codes should be present")

			// The value should be a []string, not a string
			codesSlice, ok := statusCodes.([]string)
			require.True(t, ok, "response_status_codes should be []string, got %T", statusCodes)
			assert.Equal(t, tt.wantCodeCount, len(codesSlice), "Expected %d status codes", tt.wantCodeCount)
			assert.Equal(t, tt.wantCodes, codesSlice, "Status codes should match")

			// Also verify it serializes to a JSON array
			jsonBytes, err := json.Marshal(retryRule)
			require.NoError(t, err, "JSON marshal should not error")

			var parsed map[string]interface{}
			err = json.Unmarshal(jsonBytes, &parsed)
			require.NoError(t, err, "JSON unmarshal should not error")

			jsonCodes, ok := parsed["response_status_codes"].([]interface{})
			require.True(t, ok, "JSON response_status_codes should be an array, got %T", parsed["response_status_codes"])
			assert.Len(t, jsonCodes, tt.wantCodeCount, "JSON array should have %d elements", tt.wantCodeCount)
		})
	}
}

// TestBug1_UpsertBuildRequestNoDestinationFlags tests that when upserting with
// only rule flags, the request uses destination_id (not a full destination object
// with potentially incomplete auth config).
//
// Reproduces: https://github.com/hookdeck/hookdeck-cli/issues/209 Bug 1
func TestBug1_UpsertBuildRequestNoDestinationFlags(t *testing.T) {
	cu := &connectionUpsertCmd{
		connectionCreateCmd: &connectionCreateCmd{},
	}
	cu.name = "test-conn"
	// Only set rule flags, no source/destination flags
	cu.RuleRetryStrategy = "linear"
	cu.RuleRetryCount = 3

	// Simulate existing connection with destination that has auth
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
				"auth": map[string]interface{}{
					"access_key_id":     "AKIA...",
					"secret_access_key": "...",
					"region":            "us-east-1",
					"service":           "execute-api",
				},
			},
		},
	}

	req, err := cu.buildUpsertRequest(existing, true)
	require.NoError(t, err, "buildUpsertRequest should not error")

	// The request should use DestinationID to reference the existing destination,
	// NOT Destination with a full config (which could include auth_type without credentials)
	assert.NotNil(t, req.DestinationID, "Should use DestinationID to reference existing destination")
	assert.Equal(t, "dst_123", *req.DestinationID, "DestinationID should match existing")
	assert.Nil(t, req.Destination, "Should NOT send full Destination object when no destination flags provided")

	// Source should also be preserved by ID
	assert.NotNil(t, req.SourceID, "Should use SourceID to reference existing source")
	assert.Equal(t, "src_123", *req.SourceID, "SourceID should match existing")
	assert.Nil(t, req.Source, "Should NOT send full Source object when no source flags provided")

	// Rules should be present
	assert.NotEmpty(t, req.Rules, "Rules should be present")
}

// TestBug1_HasAnyDestinationFlagDefaultCliPath tests that hasAnyDestinationFlag
// does not return true just because destination-cli-path has its default value.
//
// Reproduces: https://github.com/hookdeck/hookdeck-cli/issues/209 Bug 1 root cause
func TestBug1_HasAnyDestinationFlagDefaultCliPath(t *testing.T) {
	cu := &connectionUpsertCmd{
		connectionCreateCmd: &connectionCreateCmd{},
	}
	// Don't set any destination flags - leave defaults
	// The CLI path default is "/" in the flag definition, but for upsert it should be ""

	result := cu.hasAnyDestinationFlag()
	assert.False(t, result, "hasAnyDestinationFlag should return false when no destination flags are explicitly set")
}

// TestBug2_ValidateSourceFlagsNameOnly tests that validateSourceFlags does NOT
// require --source-type when only --source-name is provided during upsert.
//
// Reproduces: https://github.com/hookdeck/hookdeck-cli/issues/209 Bug 2
func TestBug2_ValidateSourceFlagsNameOnly(t *testing.T) {
	cu := &connectionUpsertCmd{
		connectionCreateCmd: &connectionCreateCmd{},
	}
	cu.sourceName = "my-source"
	// sourceType is intentionally empty

	err := cu.validateSourceFlags()
	assert.NoError(t, err, "validateSourceFlags should NOT require --source-type when only --source-name is provided during upsert")
}

func strPtr(s string) *string {
	return &s
}

package hookdeck

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The API returns "data" in several shapes. Anything this fails to decode is
// reported to the user as a raw JSON dump, so the tolerance is the feature.
func TestErrorResponseDecodesEveryDataShape(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "message only",
			body: `{"message":"validation error"}`,
			want: "validation error",
		},
		{
			name: "field detail as an array",
			body: `{"message":"validation error","data":["topic is invalid"]}`,
			want: "validation error: topic is invalid",
		},
		{
			// The shape returned when a resource or a project-level feature is
			// missing. Before this decode existed the reader got the whole
			// envelope instead of the sentence inside it.
			name: "message nested under data",
			body: `{"data":{"message":"Portal not configured for this project"},"status":404,"code":"NOT_FOUND","model":null}`,
			want: "Portal not configured for this project",
		},
		{
			name: "rejected value named under data",
			body: `{"code":"CUSTOM_DOMAIN_INVALID","status":429,"message":"Custom domain is an invalid hostname","data":{"hostname":"portal.example.com"}}`,
			want: "Custom domain is an invalid hostname: hostname: portal.example.com",
		},
		{
			name: "data as a bare string",
			body: `{"message":"nope","data":"because"}`,
			want: "nope: because",
		},
		{
			name: "data as a null",
			body: `{"message":"nope","data":null}`,
			want: "nope",
		},
		{
			name: "no message at all",
			body: `{"data":["topic is invalid"]}`,
			want: "topic is invalid",
		},
		{
			name: "non-string entries are still rendered",
			body: `{"message":"nope","data":{"limit":5}}`,
			want: "nope: limit: 5",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var response ErrorResponse
			require.NoError(t, json.Unmarshal([]byte(tc.body), &response))
			assert.Equal(t, tc.want, response.Detail())
		})
	}
}

// Several object keys are rendered in a stable order, so the message does not
// change between runs.
func TestErrorResponseOrdersFieldDetail(t *testing.T) {
	var response ErrorResponse
	require.NoError(t, json.Unmarshal(
		[]byte(`{"message":"nope","data":{"b":"2","a":"1","c":"3"}}`), &response))
	assert.Equal(t, []string{"a: 1", "b: 2", "c: 3"}, response.Data)
}

// Tests elsewhere in this package serialise ErrorResponse to build stub
// responses, so the round trip has to survive the custom decoder.
func TestErrorResponseRoundTrips(t *testing.T) {
	encoded, err := json.Marshal(ErrorResponse{Message: "test error", Data: []string{"a", "b"}})
	require.NoError(t, err)

	var decoded ErrorResponse
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	assert.Equal(t, "test error", decoded.Message)
	assert.Equal(t, []string{"a", "b"}, decoded.Data)
}

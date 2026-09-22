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
		{
			// Ported from main's apiErrorMessage coverage. A validation failure
			// carries no top-level message and the one useful line sits in
			// data[], while the envelope around it is transport bookkeeping.
			// Pinned separately below that none of it leaks.
			name: "422 validation body puts the message in data[]",
			body: `{"level":"info","handled":true,"report":true,"data":["The delivery_group dimension requires a filters.destination_id filter"],"status":422,"code":"UNPROCESSABLE_ENTITY"}`,
			want: "The delivery_group dimension requires a filters.destination_id filter",
		},
		{
			name: "measure enum rejection",
			body: `{"level":"info","handled":true,"data":["measures[0] must be one of [count, accepted_count]"],"status":422}`,
			want: "measures[0] must be one of [count, accepted_count]",
		},
		{
			name: "several data entries are joined",
			body: `{"data":["first problem","second problem"],"status":422}`,
			want: "first problem; second problem",
		},
		{
			name: "data entries may be objects",
			body: `{"data":[{"message":"nested problem"}],"status":422}`,
			want: "nested problem",
		},
		{
			name: "nothing readable falls back to the caller",
			body: `{"status":500}`,
			want: "",
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

// TestErrorResponseDropsInternalFields is main's point, stated against this
// branch's decoder: whatever else happens, the transport's own bookkeeping must
// not reach an MCP client as error text.
//
// Ported from pkg/hookdeck/client_error_message_test.go, which tested the
// apiErrorMessage helper that ErrorResponse.Detail supersedes. The two differ
// deliberately when a body carries both a message and data: apiErrorMessage
// returned the message alone, Detail appends the field detail, because the
// rejected value is usually the part the caller needs (see "rejected value
// named under data" above).
func TestErrorResponseDropsInternalFields(t *testing.T) {
	var response ErrorResponse
	require.NoError(t, json.Unmarshal([]byte(`{"level":"info","handled":true,"report":true,"data":["dimensions[0] must be [destination_id]"],"status":422,"code":"UNPROCESSABLE_ENTITY"}`), &response))

	got := response.Detail()
	for _, leaked := range []string{"level", "handled", "report", "UNPROCESSABLE_ENTITY"} {
		assert.NotContains(t, got, leaked)
	}
	assert.Equal(t, "dimensions[0] must be [destination_id]", got)
}

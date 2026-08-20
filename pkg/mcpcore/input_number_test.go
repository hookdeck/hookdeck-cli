package mcpcore

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Filters that accept "an integer or the API's operator syntax" are typed as
// string in the schema, because the operator form is a string. A model that
// sends the integer as a JSON number must still have its filter applied: String
// returns "" for a non-string, which dropped the filter silently and returned an
// unfiltered result that looked like a filtered one.
func TestNumberOrStringAcceptsBothForms(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want string
	}{
		{"json number zero", `{"events_count": 0}`, "0"},
		{"json number", `{"events_count": 12}`, "12"},
		{"quoted number", `{"events_count": "12"}`, "12"},
		{"operator syntax", `{"events_count": "[gte]=5"}`, "[gte]=5"},
		{"absent", `{}`, ""},
		{"null", `{"events_count": null}`, ""},
		{"bool is not a number", `{"events_count": true}`, ""},
		{"non-integral", `{"events_count": 1.5}`, "1.5"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var in Input
			require := json.Unmarshal([]byte(tc.raw), &in)
			assert.NoError(t, require)
			assert.Equal(t, tc.want, in.NumberOrString("events_count"))
		})
	}
}

// Zero is the case that matters most: "which requests delivered nothing" is
// events_count: 0, and a float formatter would render it "0.000000", which the
// API does not read as zero.
func TestNumberOrStringRendersZeroAsZero(t *testing.T) {
	var in Input
	assert.NoError(t, json.Unmarshal([]byte(`{"events_count": 0}`), &in))
	assert.Equal(t, "0", in.NumberOrString("events_count"))
	assert.NotEqual(t, "", in.NumberOrString("events_count"),
		"a zero filter must survive; dropping it returns every record unfiltered")
}

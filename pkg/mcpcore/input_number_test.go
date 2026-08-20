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

// The same trait for boolean filters. verified: "false" was read as "no filter"
// rather than "unverified only", so the tool listed every request and the
// caller had no way to tell its filter had been ignored.
func TestBoolOrStringAcceptsBothForms(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want *bool
	}{
		{"json true", `{"verified": true}`, boolPtr(true)},
		{"json false", `{"verified": false}`, boolPtr(false)},
		{"quoted true", `{"verified": "true"}`, boolPtr(true)},
		{"quoted false", `{"verified": "false"}`, boolPtr(false)},
		{"ParseBool numeric forms", `{"verified": "0"}`, boolPtr(false)},
		{"ParseBool single letter", `{"verified": "T"}`, boolPtr(true)},
		{"absent", `{}`, nil},
		{"null", `{"verified": null}`, nil},
		{"unparseable stays absent", `{"verified": "yes"}`, nil},
		{"number is not a bool", `{"verified": 1}`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var in Input
			assert.NoError(t, json.Unmarshal([]byte(tc.raw), &in))
			got := in.BoolOrString("verified")
			if tc.want == nil {
				assert.Nil(t, got)
				return
			}
			if assert.NotNil(t, got, "a valid filter must not be dropped") {
				assert.Equal(t, *tc.want, *got)
			}
		})
	}
}

// Bool reads the same values; a quoted "true" must not read as false.
func TestBoolAcceptsQuotedForm(t *testing.T) {
	var in Input
	assert.NoError(t, json.Unmarshal([]byte(`{"reauth": "true", "other": "false"}`), &in))
	assert.True(t, in.Bool("reauth"))
	assert.False(t, in.Bool("other"))
	assert.False(t, in.Bool("absent"))
}

// limit: "5" returned the default, which is the API's page size rather than the
// one the caller asked for — a quietly different result set.
func TestIntAcceptsNumericString(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want int
	}{
		{"json number", `{"limit": 5}`, 5},
		{"quoted number", `{"limit": "5"}`, 5},
		{"quoted zero", `{"limit": "0"}`, 0},
		{"absent falls back", `{}`, 7},
		{"unparseable falls back", `{"limit": "many"}`, 7},
		{"bool falls back", `{"limit": true}`, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var in Input
			assert.NoError(t, json.Unmarshal([]byte(tc.raw), &in))
			assert.Equal(t, tc.want, in.Int("limit", 7))
		})
	}
}

func boolPtr(b bool) *bool { return &b }

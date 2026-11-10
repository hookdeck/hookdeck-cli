package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --rule-retry-response-status-codes accepts the same expressions as the API's RetryRule
// response_status_codes pattern (and --rules JSON): codes, ranges, comparisons, exclusions.
func TestRetryResponseStatusCodesAcceptsAPIExpressions(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{name: "single code", value: "429", want: []string{"429"}},
		{name: "code and range", value: "429,500-599", want: []string{"429", "500-599"}},
		{name: "comparisons", value: ">400,>=500,<=499,<300,=404", want: []string{">400", ">=500", "<=499", "<300", "=404"}},
		{name: "exclusion", value: "500-599,!501", want: []string{"500-599", "!501"}},
		{name: "whitespace and empty entries trimmed", value: " 429 , 500-599 ,, ", want: []string{"429", "500-599"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := parseRuleFlags(t, []string{
				"--rule-retry-strategy", "linear",
				"--rule-retry-response-status-codes", tt.value,
			})
			rules, err := buildConnectionRules(f)
			require.NoError(t, err)
			require.Len(t, rules, 1)
			assert.Equal(t, "retry", rules[0]["type"])
			assert.Equal(t, tt.want, rules[0]["response_status_codes"])
		})
	}
}

func TestRetryResponseStatusCodesRejectsInvalidExpressions(t *testing.T) {
	for _, value := range []string{
		"abc",     // not a code
		"100",     // API only accepts 200-599
		"600",     // out of range
		"500-",    // incomplete range
		"500-6xx", // malformed range end
		">>>400",  // too many comparison characters
		"!>400",   // exclusion only applies to a single code
		"4xx",     // wildcard is not an API expression
	} {
		t.Run(value, func(t *testing.T) {
			f := parseRuleFlags(t, []string{
				"--rule-retry-strategy", "linear",
				"--rule-retry-response-status-codes", value,
			})
			_, err := buildConnectionRules(f)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "--rule-retry-response-status-codes")
			assert.Contains(t, err.Error(), value)
		})
	}
}

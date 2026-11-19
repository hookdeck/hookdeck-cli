package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestConnectionUpdateRejectsOrphanRetryFlags covers rule flags given without
// --rule-retry-strategy. create and upsert reject them; update silently dropped
// them, sent nothing, and printed the stored connection as if it had applied
// the change.
func TestConnectionUpdateRejectsOrphanRetryFlags(t *testing.T) {
	for _, flags := range [][]string{
		{"--rule-retry-count", "5"},
		{"--rule-retry-interval", "1000"},
		{"--rule-retry-response-status-codes", "500"},
	} {
		flags := flags
		t.Run(flags[0], func(t *testing.T) {
			stub, err := runConnectionCommand(t, storedWithSides(),
				append([]string{"gateway", "connection", "update", "web_1"}, flags...)...)

			require.Error(t, err)
			require.Contains(t, err.Error(), "--rule-retry-strategy is required")
			require.Empty(t, stub.writes())
		})
	}
}

// TestConnectionUpdateRejectsInvalidRuleFlags checks that update runs the same
// rule validation as create: an unknown strategy and a transform rule without
// a name are both rejected before any request is made.
func TestConnectionUpdateRejectsInvalidRuleFlags(t *testing.T) {
	cases := map[string][]string{
		"bad strategy":      {"--rule-retry-strategy", "fibonacci"},
		"transform no name": {"--rule-transform-code", "return 1"},
		"filter with rules": {"--rules", "[]", "--rule-filter-body", "{}"},
	}
	for name, flags := range cases {
		flags := flags
		t.Run(name, func(t *testing.T) {
			stub, err := runConnectionCommand(t, storedWithSides(),
				append([]string{"gateway", "connection", "update", "web_1"}, flags...)...)

			require.Error(t, err)
			require.Empty(t, stub.writes())
		})
	}
}

// TestConnectionUpdateRejectsNegativeRuleFlags covers negative values given on
// their own. The "has rule flags" checks only counted values > 0, so a lone
// negative --rule-delay, --rule-deduplicate-window or retry count/interval
// skipped validation and was silently ignored; the "must be a positive
// integer" errors only appeared when another rule flag was also set.
func TestConnectionUpdateRejectsNegativeRuleFlags(t *testing.T) {
	cases := map[string]struct {
		flags []string
		want  string
	}{
		"delay":             {[]string{"--rule-delay", "-5"}, "--rule-delay must be a positive integer"},
		"deduplicate":       {[]string{"--rule-deduplicate-window", "-5"}, "--rule-deduplicate-window must be a positive integer"},
		"retry count":       {[]string{"--rule-retry-strategy", "linear", "--rule-retry-count", "-1"}, "--rule-retry-count must be a positive integer"},
		"retry interval":    {[]string{"--rule-retry-strategy", "linear", "--rule-retry-interval", "-1"}, "--rule-retry-interval must be a positive integer"},
		"orphan retry cnt":  {[]string{"--rule-retry-count", "-1"}, "--rule-retry-strategy is required"},
		"orphan retry intv": {[]string{"--rule-retry-interval", "-1"}, "--rule-retry-strategy is required"},
	}
	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			stub, err := runConnectionCommand(t, storedWithSides(),
				append([]string{"gateway", "connection", "update", "web_1"}, tc.flags...)...)

			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
			require.Empty(t, stub.writes())
		})
	}
}

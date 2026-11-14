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

package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// parseRuleFlags binds the rule flags to a throwaway command and parses args,
// so flag values are set the same way cobra sets them at runtime.
func parseRuleFlags(t *testing.T, args []string) *connectionRuleFlags {
	t.Helper()
	f := &connectionRuleFlags{}
	cmd := &cobra.Command{Use: "test"}
	addConnectionRuleFlags(cmd, f)
	require.NoError(t, cmd.ParseFlags(args))
	return f
}

func ruleTypes(rules []hookdeck.Rule) []string {
	types := make([]string, 0, len(rules))
	for _, r := range rules {
		types = append(types, r["type"].(string))
	}
	return types
}

// TestBuildConnectionRulesFollowsFlagOrder verifies that rules built from
// --rule-<type>-* flags follow the position of the first flag for each type.
// Filter, transform and deduplicate run in rules array order, so a fixed
// order made filter-before-transform impossible to express with flags.
func TestBuildConnectionRulesFollowsFlagOrder(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "filter before transform",
			args: []string{"--rule-filter-body", `{"type":"order"}`, "--rule-transform-name", "tx1"},
			want: []string{"filter", "transform"},
		},
		{
			name: "transform before filter",
			args: []string{"--rule-transform-name", "tx1", "--rule-filter-body", `{"type":"order"}`},
			want: []string{"transform", "filter"},
		},
		{
			name: "first flag of a type sets its position",
			args: []string{
				"--rule-filter-body", `{"type":"order"}`,
				"--rule-transform-name", "tx1",
				"--rule-filter-headers", `{"x-topic":"orders/create"}`,
			},
			want: []string{"filter", "transform"},
		},
		{
			name: "all five types in flag order",
			args: []string{
				"--rule-retry-strategy", "exponential",
				"--rule-filter-body", `{"type":"order"}`,
				"--rule-delay", "1000",
				"--rule-transform-name", "tx1",
				"--rule-deduplicate-window", "60",
			},
			want: []string{"retry", "filter", "delay", "transform", "deduplicate"},
		},
		{
			name: "retry position set by a non-strategy retry flag",
			args: []string{"--rule-retry-count", "3", "--rule-filter-body", `{"type":"order"}`, "--rule-retry-strategy", "linear"},
			want: []string{"retry", "filter"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules, err := buildConnectionRules(parseRuleFlags(t, tt.args))
			require.NoError(t, err)
			assert.Equal(t, tt.want, ruleTypes(rules))
		})
	}
}

// TestBuildConnectionRulesDefaultOrderWithoutFlags verifies the fallback order
// when connectionRuleFlags is populated directly rather than parsed from flags.
func TestBuildConnectionRulesDefaultOrderWithoutFlags(t *testing.T) {
	flags := connectionRuleFlags{
		RuleRetryStrategy:     "linear",
		RuleFilterBody:        `{"type":"order"}`,
		RuleTransformName:     "tx1",
		RuleDelay:             1000,
		RuleDeduplicateWindow: 60,
	}
	rules, err := buildConnectionRules(&flags)
	require.NoError(t, err)
	assert.Equal(t, []string{"deduplicate", "transform", "filter", "delay", "retry"}, ruleTypes(rules))
}

// TestConnectionCommandsRuleFlagOrder verifies that create, update and upsert
// all record rule flag order through their own flag sets.
func TestConnectionCommandsRuleFlagOrder(t *testing.T) {
	args := []string{"--rule-filter-body", `{"type":"order"}`, "--rule-transform-name", "tx1"}

	create := newConnectionCreateCmd()
	require.NoError(t, create.cmd.ParseFlags(args))
	rules, err := buildConnectionRules(&create.connectionRuleFlags)
	require.NoError(t, err)
	assert.Equal(t, []string{"filter", "transform"}, ruleTypes(rules), "create")

	update := newConnectionUpdateCmd()
	require.NoError(t, update.cmd.ParseFlags(args))
	rules, err = buildConnectionRules(&update.connectionRuleFlags)
	require.NoError(t, err)
	assert.Equal(t, []string{"filter", "transform"}, ruleTypes(rules), "update")

	upsert := newConnectionUpsertCmd()
	require.NoError(t, upsert.cmd.ParseFlags(args))
	rules, err = buildConnectionRules(&upsert.connectionCreateCmd.connectionRuleFlags)
	require.NoError(t, err)
	assert.Equal(t, []string{"filter", "transform"}, ruleTypes(rules), "upsert")
}

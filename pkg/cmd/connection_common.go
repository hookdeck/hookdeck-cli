package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// shouldShowConnectionDeprecation returns true when the user invoked the
// root-level alias (hookdeck connection / hookdeck connections) and we
// should print a deprecation notice. Returns false when:
// - Invoked under gateway (hookdeck gateway connection ...)
// - Output is JSON (--output json or --output=json), so the notice would pollute machine output
// - Any future silent/quiet flag is set (none today; add here when introduced)
func shouldShowConnectionDeprecation() bool {
	args := os.Args
	if len(args) < 2 {
		return false
	}
	first := args[1]
	if first != "connection" && first != "connections" {
		return false // under gateway or another command
	}
	for i, a := range args {
		if a == "--output" && i+1 < len(args) && strings.TrimSpace(args[i+1]) == "json" {
			return false
		}
		if strings.HasPrefix(a, "--output=") && strings.TrimSpace(strings.TrimPrefix(a, "--output=")) == "json" {
			return false
		}
		// If a global silent/quiet flag is added later, check for it here and return false
	}
	return true
}

// connectionRuleFlags holds rule-related flags shared by connection create, update, and upsert.
// Used to avoid duplicating flag definitions and rule-building logic.
type connectionRuleFlags struct {
	Rules     string
	RulesFile string

	RuleRetryStrategy           string
	RuleRetryCount              int
	RuleRetryInterval           int
	RuleRetryResponseStatusCode string

	RuleFilterBody    string
	RuleFilterHeaders string
	RuleFilterQuery   string
	RuleFilterPath    string

	RuleTransformName string
	RuleTransformCode string
	RuleTransformEnv  string

	RuleDelay int

	RuleDeduplicateWindow        int
	RuleDeduplicateIncludeFields string
	RuleDeduplicateExcludeFields string
}

// addConnectionRuleFlags binds rule flags to cmd. Pass a pointer to the flags struct
// (e.g. embedded in connectionCreateCmd, connectionUpdateCmd) so values are populated.
func addConnectionRuleFlags(cmd *cobra.Command, f *connectionRuleFlags) {
	cmd.Flags().StringVar(&f.Rules, "rules", "", "JSON string representing the entire rules array")
	cmd.Flags().StringVar(&f.RulesFile, "rules-file", "", "Path to a JSON file containing the rules array")

	cmd.Flags().StringVar(&f.RuleRetryStrategy, "rule-retry-strategy", "", "Retry strategy (linear, exponential)")
	cmd.Flags().IntVar(&f.RuleRetryCount, "rule-retry-count", 0, "Number of retry attempts")
	cmd.Flags().IntVar(&f.RuleRetryInterval, "rule-retry-interval", 0, "Interval between retries in milliseconds")
	cmd.Flags().StringVar(&f.RuleRetryResponseStatusCode, "rule-retry-response-status-codes", "", "Comma-separated HTTP status codes to retry on")

	cmd.Flags().StringVar(&f.RuleFilterBody, "rule-filter-body", "", "JQ expression to filter on request body")
	cmd.Flags().StringVar(&f.RuleFilterHeaders, "rule-filter-headers", "", "JQ expression to filter on request headers")
	cmd.Flags().StringVar(&f.RuleFilterQuery, "rule-filter-query", "", "JQ expression to filter on request query parameters")
	cmd.Flags().StringVar(&f.RuleFilterPath, "rule-filter-path", "", "JQ expression to filter on request path")

	cmd.Flags().StringVar(&f.RuleTransformName, "rule-transform-name", "", "Name or ID of the transformation to apply")
	cmd.Flags().StringVar(&f.RuleTransformCode, "rule-transform-code", "", "Transformation code (if creating inline)")
	cmd.Flags().StringVar(&f.RuleTransformEnv, "rule-transform-env", "", "JSON string representing environment variables for transformation")

	cmd.Flags().IntVar(&f.RuleDelay, "rule-delay", 0, "Delay in milliseconds")

	cmd.Flags().IntVar(&f.RuleDeduplicateWindow, "rule-deduplicate-window", 0, "Time window in seconds for deduplication")
	cmd.Flags().StringVar(&f.RuleDeduplicateIncludeFields, "rule-deduplicate-include-fields", "", "Comma-separated list of fields to include for deduplication")
	cmd.Flags().StringVar(&f.RuleDeduplicateExcludeFields, "rule-deduplicate-exclude-fields", "", "Comma-separated list of fields to exclude for deduplication")
}

// buildConnectionRules builds a slice of rules from connectionRuleFlags.
// If rulesStr or rulesFile is non-empty, those are parsed as JSON and returned;
// otherwise individual rule flags are assembled into rules.
// Shared by connection update and (for consistency) can be used by create/upsert.
func buildConnectionRules(f *connectionRuleFlags) ([]hookdeck.Rule, error) {
	if f.Rules != "" {
		var rules []hookdeck.Rule
		if err := json.Unmarshal([]byte(f.Rules), &rules); err != nil {
			return nil, fmt.Errorf("invalid JSON for --rules: %w", err)
		}
		return rules, nil
	}

	if f.RulesFile != "" {
		data, err := os.ReadFile(f.RulesFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read rules file: %w", err)
		}
		var rules []hookdeck.Rule
		if err := json.Unmarshal(data, &rules); err != nil {
			return nil, fmt.Errorf("invalid JSON in rules file: %w", err)
		}
		return rules, nil
	}

	// Build each rule type (order matches create: deduplicate -> transform -> filter -> delay -> retry)
	var rules []hookdeck.Rule

	if f.RuleDeduplicateWindow > 0 {
		rule := hookdeck.Rule{
			"type":   "deduplicate",
			"window": f.RuleDeduplicateWindow,
		}
		if f.RuleDeduplicateIncludeFields != "" {
			rule["include_fields"] = strings.Split(f.RuleDeduplicateIncludeFields, ",")
		}
		if f.RuleDeduplicateExcludeFields != "" {
			rule["exclude_fields"] = strings.Split(f.RuleDeduplicateExcludeFields, ",")
		}
		rules = append(rules, rule)
	}

	hasTransform := f.RuleTransformName != "" || f.RuleTransformCode != "" || f.RuleTransformEnv != ""
	if hasTransform {
		rule := hookdeck.Rule{"type": "transform"}
		transformConfig := make(map[string]interface{})
		if f.RuleTransformName != "" {
			transformConfig["name"] = f.RuleTransformName
		}
		if f.RuleTransformCode != "" {
			transformConfig["code"] = f.RuleTransformCode
		}
		if f.RuleTransformEnv != "" {
			var env map[string]interface{}
			if err := json.Unmarshal([]byte(f.RuleTransformEnv), &env); err != nil {
				return nil, fmt.Errorf("invalid JSON for --rule-transform-env: %w", err)
			}
			transformConfig["env"] = env
		}
		rule["transformation"] = transformConfig
		rules = append(rules, rule)
	}

	if f.RuleFilterBody != "" || f.RuleFilterHeaders != "" || f.RuleFilterQuery != "" || f.RuleFilterPath != "" {
		rule := hookdeck.Rule{"type": "filter"}
		if f.RuleFilterBody != "" {
			rule["body"] = f.RuleFilterBody
		}
		if f.RuleFilterHeaders != "" {
			rule["headers"] = f.RuleFilterHeaders
		}
		if f.RuleFilterQuery != "" {
			rule["query"] = f.RuleFilterQuery
		}
		if f.RuleFilterPath != "" {
			rule["path"] = f.RuleFilterPath
		}
		rules = append(rules, rule)
	}

	if f.RuleDelay > 0 {
		rules = append(rules, hookdeck.Rule{
			"type":  "delay",
			"delay": f.RuleDelay,
		})
	}

	if f.RuleRetryStrategy != "" {
		rule := hookdeck.Rule{
			"type":     "retry",
			"strategy": f.RuleRetryStrategy,
		}
		if f.RuleRetryCount > 0 {
			rule["count"] = f.RuleRetryCount
		}
		if f.RuleRetryInterval > 0 {
			rule["interval"] = f.RuleRetryInterval
		}
		if f.RuleRetryResponseStatusCode != "" {
			codes := strings.Split(f.RuleRetryResponseStatusCode, ",")
			for i := range codes {
				codes[i] = strings.TrimSpace(codes[i])
			}
			rule["response_status_codes"] = codes
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

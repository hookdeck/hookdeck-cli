package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

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

	// ruleOrder records rule types in the order their first --rule-<type>-* flag
	// appeared on the command line. Filter, transform and deduplicate run in
	// rules array order, so the flags must not impose a fixed order.
	ruleOrder []string
}

// defaultRuleOrder is used for rule types with no recorded flag position, such as
// when connectionRuleFlags is populated directly rather than from the command line.
var defaultRuleOrder = []string{"deduplicate", "transform", "filter", "delay", "retry"}

// ruleFlagTypes maps each individual rule flag to the rule type it configures.
var ruleFlagTypes = map[string]string{
	"rule-retry-strategy":              "retry",
	"rule-retry-count":                 "retry",
	"rule-retry-interval":              "retry",
	"rule-retry-response-status-codes": "retry",
	"rule-filter-body":                 "filter",
	"rule-filter-headers":              "filter",
	"rule-filter-query":                "filter",
	"rule-filter-path":                 "filter",
	"rule-transform-name":              "transform",
	"rule-transform-code":              "transform",
	"rule-transform-env":               "transform",
	"rule-delay":                       "delay",
	"rule-deduplicate-window":          "deduplicate",
	"rule-deduplicate-include-fields":  "deduplicate",
	"rule-deduplicate-exclude-fields":  "deduplicate",
}

// recordRuleType appends ruleType to the recorded order unless it is already present.
func (f *connectionRuleFlags) recordRuleType(ruleType string) {
	for _, t := range f.ruleOrder {
		if t == ruleType {
			return
		}
	}
	f.ruleOrder = append(f.ruleOrder, ruleType)
}

// orderTrackingValue wraps a flag value so that setting it records the flag's rule type.
type orderTrackingValue struct {
	pflag.Value
	onSet func()
}

func (v *orderTrackingValue) Set(s string) error {
	if err := v.Value.Set(s); err != nil {
		return err
	}
	v.onSet()
	return nil
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

	cmd.Flags().StringVar(&f.RuleFilterBody, "rule-filter-body", "", "Filter on request body using Hookdeck filter syntax (JSON)")
	cmd.Flags().StringVar(&f.RuleFilterHeaders, "rule-filter-headers", "", "Filter on request headers using Hookdeck filter syntax (JSON)")
	cmd.Flags().StringVar(&f.RuleFilterQuery, "rule-filter-query", "", "Filter on request query parameters using Hookdeck filter syntax (JSON)")
	cmd.Flags().StringVar(&f.RuleFilterPath, "rule-filter-path", "", "Filter on request path using Hookdeck filter syntax (JSON)")

	cmd.Flags().StringVar(&f.RuleTransformName, "rule-transform-name", "", "Name or ID of the transformation to apply")
	cmd.Flags().StringVar(&f.RuleTransformCode, "rule-transform-code", "", "Transformation code (if creating inline)")
	cmd.Flags().StringVar(&f.RuleTransformEnv, "rule-transform-env", "", "JSON string representing environment variables for transformation")

	cmd.Flags().IntVar(&f.RuleDelay, "rule-delay", 0, "Delay in milliseconds")

	cmd.Flags().IntVar(&f.RuleDeduplicateWindow, "rule-deduplicate-window", 0, "Time window in seconds for deduplication")
	cmd.Flags().StringVar(&f.RuleDeduplicateIncludeFields, "rule-deduplicate-include-fields", "", "Comma-separated list of fields to include for deduplication")
	cmd.Flags().StringVar(&f.RuleDeduplicateExcludeFields, "rule-deduplicate-exclude-fields", "", "Comma-separated list of fields to exclude for deduplication")

	for name, ruleType := range ruleFlagTypes {
		flag := cmd.Flags().Lookup(name)
		ruleType := ruleType
		flag.Value = &orderTrackingValue{Value: flag.Value, onSet: func() { f.recordRuleType(ruleType) }}
	}
}

// buildConnectionRules builds a slice of rules from connectionRuleFlags.
// If rulesStr or rulesFile is non-empty, those are parsed as JSON and returned;
// otherwise individual rule flags are assembled into rules, ordered by the
// position of the first flag for each rule type.
// Shared by connection update and (for consistency) can be used by create/upsert.
func buildConnectionRules(f *connectionRuleFlags) ([]hookdeck.Rule, error) {
	if f.Rules != "" {
		var rules []hookdeck.Rule
		if err := json.Unmarshal([]byte(f.Rules), &rules); err != nil {
			return nil, fmt.Errorf("invalid JSON for --rules: %w", err)
		}
		return normalizeRulesForAPI(rules), nil
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
		return normalizeRulesForAPI(rules), nil
	}

	// Build each rule type, then order them by flag position
	built := make(map[string]hookdeck.Rule)

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
		built["deduplicate"] = rule
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
		built["transform"] = rule
	}

	if f.RuleFilterBody != "" || f.RuleFilterHeaders != "" || f.RuleFilterQuery != "" || f.RuleFilterPath != "" {
		rule := hookdeck.Rule{"type": "filter"}
		if f.RuleFilterBody != "" {
			rule["body"] = parseJSONOrString(f.RuleFilterBody)
		}
		if f.RuleFilterHeaders != "" {
			rule["headers"] = parseJSONOrString(f.RuleFilterHeaders)
		}
		if f.RuleFilterQuery != "" {
			rule["query"] = parseJSONOrString(f.RuleFilterQuery)
		}
		if f.RuleFilterPath != "" {
			rule["path"] = parseJSONOrString(f.RuleFilterPath)
		}
		built["filter"] = rule
	}

	if f.RuleDelay > 0 {
		built["delay"] = hookdeck.Rule{
			"type":  "delay",
			"delay": f.RuleDelay,
		}
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
		// API expects response_status_codes as []string (RetryRule schema)
		if f.RuleRetryResponseStatusCode != "" {
			parts := strings.Split(f.RuleRetryResponseStatusCode, ",")
			strCodes := make([]string, 0, len(parts))
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				n, err := strconv.Atoi(part)
				if err != nil {
					return nil, fmt.Errorf("invalid HTTP status code %q in --rule-retry-response-status-codes: must be an integer", part)
				}
				if n < 100 || n > 599 {
					return nil, fmt.Errorf("invalid HTTP status code %d in --rule-retry-response-status-codes: must be between 100 and 599", n)
				}
				strCodes = append(strCodes, part)
			}
			rule["response_status_codes"] = strCodes
		}
		built["retry"] = rule
	}

	var rules []hookdeck.Rule
	for _, ruleType := range append(append([]string{}, f.ruleOrder...), defaultRuleOrder...) {
		if rule, ok := built[ruleType]; ok {
			rules = append(rules, rule)
			delete(built, ruleType)
		}
	}

	return rules, nil
}

// normalizeRulesForAPI ensures rules match the API schema: RetryRule.response_status_codes
// must be []string; FilterRule body/headers may be string or object.
func normalizeRulesForAPI(rules []hookdeck.Rule) []hookdeck.Rule {
	out := make([]hookdeck.Rule, len(rules))
	for i, r := range rules {
		out[i] = make(hookdeck.Rule)
		for k, v := range r {
			out[i][k] = v
		}
		if r["type"] == "retry" {
			if codes, ok := r["response_status_codes"].([]interface{}); ok && len(codes) > 0 {
				strCodes := make([]string, 0, len(codes))
				for _, c := range codes {
					switch v := c.(type) {
					case string:
						strCodes = append(strCodes, v)
					case float64:
						strCodes = append(strCodes, strconv.Itoa(int(v)))
					case int:
						strCodes = append(strCodes, strconv.Itoa(v))
					default:
						strCodes = append(strCodes, fmt.Sprintf("%v", c))
					}
				}
				out[i]["response_status_codes"] = strCodes
			}
		}
	}
	return out
}

// parseJSONOrString attempts to parse s as a JSON object or array. Only values
// starting with '{' or '[' (after trimming whitespace) are candidates for
// parsing; bare primitives like "order", 123, or true are returned as-is so
// that plain strings are never misinterpreted.
func parseJSONOrString(s string) interface{} {
	trimmed := strings.TrimSpace(s)
	if len(trimmed) == 0 {
		return s
	}
	if trimmed[0] != '{' && trimmed[0] != '[' {
		return s
	}
	var v interface{}
	if err := json.Unmarshal([]byte(s), &v); err == nil {
		return v
	}
	return s
}

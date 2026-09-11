package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// destinationConfigFlags holds destination config flags for create/upsert/update.
// Used by destination create, upsert, update. When both --config/--config-file and
// individual flags are set, --config/--config-file take precedence.
type destinationConfigFlags struct {
	URL                     string
	CliPath                 string
	AuthMethod              string
	BearerToken             string
	BasicAuthUser           string
	BasicAuthPass           string
	APIKey                  string
	APIKeyHeader            string
	APIKeyTo                string
	CustomSignatureSecret   string
	CustomSignatureKey      string
	RateLimit               int
	RateLimitPeriod         string
	DeliveryGroupKey        string
	DeliveryGroupRate       int
	DeliveryGroupRatePeriod string
	DeliveryGroupOverrides  string
	PathForwardingDisabled  *bool
	HTTPMethod              string
}

func addDestinationDeliveryPolicyFlags(cmd *cobra.Command, flags *destinationConfigFlags) {
	cmd.Flags().IntVar(&flags.RateLimit, "rate-limit", 0, "Rate limit (requests per period)")
	cmd.Flags().StringVar(&flags.RateLimitPeriod, "rate-limit-period", "", "Rate limit period (second, minute, hour, concurrent)")
	cmd.Flags().StringVar(&flags.DeliveryGroupKey, "delivery-group-key", "", "Payload field path used to group deliveries (for example body.customer_id)")
	cmd.Flags().IntVar(&flags.DeliveryGroupRate, "delivery-group-rate", 0, "Default maximum delivery rate for each delivery group")
	cmd.Flags().StringVar(&flags.DeliveryGroupRatePeriod, "delivery-group-rate-period", "", "Delivery group rate period (second, minute, hour)")
	cmd.Flags().StringVar(&flags.DeliveryGroupOverrides, "delivery-group-overrides", "", "JSON object of group-specific delivery rate overrides")
}

func addConnectionDestinationDeliveryPolicyFlags(cmd *cobra.Command, flags *connectionCreateCmd) {
	cmd.Flags().IntVar(&flags.DestinationRateLimit, "destination-rate-limit", 0, "Rate limit for destination (requests per period)")
	cmd.Flags().StringVar(&flags.DestinationRateLimitPeriod, "destination-rate-limit-period", "", "Rate limit period (second, minute, hour, concurrent)")
	cmd.Flags().StringVar(&flags.DestinationDeliveryGroupKey, "destination-delivery-group-key", "", "Payload field path used to group deliveries (for example body.customer_id)")
	cmd.Flags().IntVar(&flags.DestinationDeliveryGroupRate, "destination-delivery-group-rate", 0, "Default maximum delivery rate for each delivery group")
	cmd.Flags().StringVar(&flags.DestinationDeliveryGroupRatePeriod, "destination-delivery-group-rate-period", "", "Delivery group rate period (second, minute, hour)")
	cmd.Flags().StringVar(&flags.DestinationDeliveryGroupOverrides, "destination-delivery-group-overrides", "", "JSON object of group-specific delivery rate overrides")
}

func (f *destinationConfigFlags) validateDeliveryPolicyFlags(flagPrefix string) error {
	_, err := buildDeliveryPolicy(
		f.RateLimit,
		f.RateLimitPeriod,
		f.DeliveryGroupKey,
		f.DeliveryGroupRate,
		f.DeliveryGroupRatePeriod,
		f.DeliveryGroupOverrides,
		flagPrefix,
	)
	return err
}

// hasAnyDestinationConfig returns true if any individual destination config flag is set.
func (f *destinationConfigFlags) hasAnyDestinationConfig() bool {
	if f == nil {
		return false
	}
	return f.URL != "" || f.CliPath != "" || f.AuthMethod != "" ||
		f.BearerToken != "" || f.BasicAuthUser != "" || f.BasicAuthPass != "" ||
		f.APIKey != "" || f.APIKeyHeader != "" || f.CustomSignatureSecret != "" || f.CustomSignatureKey != "" ||
		f.RateLimit > 0 || f.RateLimitPeriod != "" || f.DeliveryGroupKey != "" ||
		f.DeliveryGroupRate > 0 || f.DeliveryGroupRatePeriod != "" || f.DeliveryGroupOverrides != "" ||
		f.PathForwardingDisabled != nil || f.HTTPMethod != ""
}

func buildDeliveryPolicy(rate int, period, groupKey string, groupRate int, groupRatePeriod, overridesJSON, flagPrefix string) (map[string]interface{}, error) {
	policy := make(map[string]interface{})
	if period != "" && rate <= 0 {
		return nil, fmt.Errorf("--%srate-limit must be a positive integer when rate limiting is configured", flagPrefix)
	}
	if rate > 0 {
		if period == "" {
			return nil, fmt.Errorf("--%srate-limit-period is required when --%srate-limit is set", flagPrefix, flagPrefix)
		}
		policy["rate"] = rate
		policy["period"] = period
	}

	hasGroups := groupKey != "" || groupRate > 0 || groupRatePeriod != "" || overridesJSON != ""
	if !hasGroups {
		return policy, nil
	}
	groupFlagPrefix := "--" + flagPrefix + "delivery-group-"
	if groupKey == "" {
		return nil, fmt.Errorf("%skey is required when delivery groups are configured", groupFlagPrefix)
	}
	if groupRate <= 0 {
		return nil, fmt.Errorf("%srate must be a positive integer when delivery groups are configured", groupFlagPrefix)
	}
	if groupRatePeriod == "" {
		return nil, fmt.Errorf("%srate-period is required when delivery groups are configured", groupFlagPrefix)
	}

	groups := map[string]interface{}{
		"key":         groupKey,
		"rate":        groupRate,
		"rate_period": groupRatePeriod,
	}
	if overridesJSON != "" {
		var overrides map[string]interface{}
		if err := json.Unmarshal([]byte(overridesJSON), &overrides); err != nil {
			return nil, fmt.Errorf("%soverrides must be a valid JSON object: %w", groupFlagPrefix, err)
		}
		if overrides == nil {
			return nil, fmt.Errorf("%soverrides must be a valid JSON object", groupFlagPrefix)
		}
		groups["overrides"] = overrides
	}
	policy["groups"] = groups
	return policy, nil
}

func mergeDeliveryPolicy(config map[string]interface{}, policy map[string]interface{}) {
	if len(policy) == 0 {
		return
	}
	merged := make(map[string]interface{})
	if existing, ok := config["delivery_policy"].(map[string]interface{}); ok {
		for key, value := range existing {
			merged[key] = value
		}
	}
	for key, value := range policy {
		merged[key] = value
	}
	config["delivery_policy"] = merged
}

// buildDestinationAuthConfig builds auth section for destination config from flags.
func buildDestinationAuthConfig(f *destinationConfigFlags) (map[string]interface{}, error) {
	if f == nil || f.AuthMethod == "" || f.AuthMethod == "hookdeck" {
		return nil, nil
	}
	auth := make(map[string]interface{})
	switch f.AuthMethod {
	case "bearer":
		if f.BearerToken == "" {
			return nil, fmt.Errorf("--bearer-token is required for bearer auth method")
		}
		auth["type"] = "BEARER_TOKEN"
		auth["token"] = f.BearerToken
	case "basic":
		if f.BasicAuthUser == "" || f.BasicAuthPass == "" {
			return nil, fmt.Errorf("--basic-auth-user and --basic-auth-pass are required for basic auth method")
		}
		auth["type"] = "BASIC_AUTH"
		auth["username"] = f.BasicAuthUser
		auth["password"] = f.BasicAuthPass
	case "api_key":
		if f.APIKey == "" {
			return nil, fmt.Errorf("--api-key is required for api_key auth method")
		}
		if f.APIKeyHeader == "" {
			return nil, fmt.Errorf("--api-key-header is required for api_key auth method")
		}
		auth["type"] = "API_KEY"
		auth["api_key"] = f.APIKey
		auth["key"] = f.APIKeyHeader
		to := f.APIKeyTo
		if to == "" {
			to = "header"
		}
		auth["to"] = to
	case "custom_signature":
		if f.CustomSignatureSecret == "" {
			return nil, fmt.Errorf("--custom-signature-secret is required for custom_signature auth method")
		}
		if f.CustomSignatureKey == "" {
			return nil, fmt.Errorf("--custom-signature-key is required for custom_signature auth method")
		}
		auth["type"] = "CUSTOM_SIGNATURE"
		auth["signing_secret"] = f.CustomSignatureSecret
		auth["key"] = f.CustomSignatureKey
	default:
		return nil, fmt.Errorf("unsupported destination auth method: %s (supported: hookdeck, bearer, basic, api_key, custom_signature)", f.AuthMethod)
	}
	return auth, nil
}

// buildDestinationConfigFromIndividualFlags builds destination config from flags for the given type.
func buildDestinationConfigFromIndividualFlags(destType string, f *destinationConfigFlags) (map[string]interface{}, error) {
	if f == nil {
		return make(map[string]interface{}), nil
	}
	config := make(map[string]interface{})

	authConfig, err := buildDestinationAuthConfig(f)
	if err != nil {
		return nil, err
	}
	if len(authConfig) > 0 {
		config["auth_type"] = authConfig["type"]
		auth := make(map[string]interface{})
		for k, v := range authConfig {
			if k != "type" {
				auth[k] = v
			}
		}
		config["auth"] = auth
	}

	policy, err := buildDeliveryPolicy(
		f.RateLimit,
		f.RateLimitPeriod,
		f.DeliveryGroupKey,
		f.DeliveryGroupRate,
		f.DeliveryGroupRatePeriod,
		f.DeliveryGroupOverrides,
		"",
	)
	if err != nil {
		return nil, err
	}
	mergeDeliveryPolicy(config, policy)

	switch strings.ToUpper(destType) {
	case "HTTP":
		if f.URL != "" {
			config["url"] = f.URL
		}
		if f.PathForwardingDisabled != nil {
			config["path_forwarding_disabled"] = *f.PathForwardingDisabled
		}
		if f.HTTPMethod != "" {
			valid := map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}
			method := strings.ToUpper(f.HTTPMethod)
			if !valid[method] {
				return nil, fmt.Errorf("--http-method must be one of: GET, POST, PUT, PATCH, DELETE")
			}
			config["http_method"] = method
		}
	case "CLI":
		if f.CliPath != "" {
			config["path"] = f.CliPath
		}
	case "MOCK_API":
		// no extra fields
	default:
		if destType != "" {
			return nil, fmt.Errorf("unsupported destination type: %s (supported: HTTP, CLI, MOCK_API)", destType)
		}
	}

	return config, nil
}

// buildDestinationConfigFromFlags parses destination config from --config/--config-file
// or from individual flags. When configStr or configFile is set, that takes precedence.
// destType is used when building from individual flags (HTTP requires url, etc.).
func buildDestinationConfigFromFlags(configStr, configFile, destType string, individual *destinationConfigFlags) (map[string]interface{}, error) {
	if configStr != "" {
		var out map[string]interface{}
		if err := json.Unmarshal([]byte(configStr), &out); err != nil {
			return nil, fmt.Errorf("invalid JSON in --config: %w", err)
		}
		return out, nil
	}
	if configFile != "" {
		data, err := os.ReadFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read --config-file: %w", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, fmt.Errorf("invalid JSON in config file: %w", err)
		}
		return out, nil
	}
	return buildDestinationConfigFromIndividualFlags(destType, individual)
}

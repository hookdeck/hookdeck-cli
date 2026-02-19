package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
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
	PathForwardingDisabled  *bool
	HTTPMethod              string
}

// hasAnyDestinationConfig returns true if any individual destination config flag is set.
func (f *destinationConfigFlags) hasAnyDestinationConfig() bool {
	if f == nil {
		return false
	}
	return f.URL != "" || f.CliPath != "" || f.AuthMethod != "" ||
		f.BearerToken != "" || f.BasicAuthUser != "" || f.BasicAuthPass != "" ||
		f.APIKey != "" || f.APIKeyHeader != "" || f.CustomSignatureSecret != "" || f.CustomSignatureKey != "" ||
		f.RateLimit > 0 || f.RateLimitPeriod != "" || f.PathForwardingDisabled != nil || f.HTTPMethod != ""
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

	if f.RateLimit > 0 {
		config["rate_limit"] = f.RateLimit
		if f.RateLimitPeriod == "" {
			return nil, fmt.Errorf("--rate-limit-period is required when --rate-limit is set")
		}
		config["rate_limit_period"] = f.RateLimitPeriod
	}

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

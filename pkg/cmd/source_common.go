package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/hookdeck/hookdeck-cli/pkg/cmd/sources"
)

// sourceConfigFlags holds individual source config flags (no "source-" prefix).
// Used by source create, upsert, and update. Same semantics as connection's
// --source-* flags; when both --config/--config-file and individual flags are
// set, --config/--config-file take precedence.
type sourceConfigFlags struct {
	WebhookSecret        string
	APIKey               string
	BasicAuthUser        string
	BasicAuthPass        string
	HMACSecret           string
	HMACAlgo             string
	AllowedHTTPMethods   string
	CustomResponseBody   string
	CustomResponseType   string
}

// hasAny returns true if any individual config flag is set.
func (f *sourceConfigFlags) hasAny() bool {
	if f == nil {
		return false
	}
	return f.WebhookSecret != "" || f.APIKey != "" ||
		f.BasicAuthUser != "" || f.BasicAuthPass != "" ||
		f.HMACSecret != "" || f.HMACAlgo != "" ||
		f.AllowedHTTPMethods != "" || f.CustomResponseBody != "" || f.CustomResponseType != ""
}

// flagRef returns the flag string for error messages (e.g. "" -> "--allowed-http-methods", "source-" -> "--source-allowed-http-methods").
func flagRef(prefix, name string) string {
	return "--" + prefix + name
}

// buildSourceConfigFromIndividualFlags builds source config from individual flags.
// Source-level type (WEBHOOK, STRIPE, etc.) determines which config schema applies; config.auth
// contents depend on that type (per OpenAPI SourceTypeConfig oneOf). No auth_type field—only auth.
// Shared by source create/upsert/update (prefix "") and connection create/upsert (prefix "source-").
// flagPrefix is used only in error messages so connection errors mention --source-*.
func buildSourceConfigFromIndividualFlags(f *sourceConfigFlags, flagPrefix, sourceType string) (map[string]interface{}, error) {
	if f == nil || !f.hasAny() {
		return nil, nil
	}
	config := make(map[string]interface{})
	sourceTypeUpper := strings.ToUpper(strings.TrimSpace(sourceType))

	// Auth: only config.auth; shape depends on source type (API infers from type + auth keys).
	if f.WebhookSecret != "" {
		if sourceTypeUpper == "STRIPE" {
			config["auth"] = map[string]interface{}{"webhook_secret_key": f.WebhookSecret}
		} else {
			config["auth"] = map[string]interface{}{
				"algorithm":          "sha256",
				"encoding":           "hex",
				"header_key":         "x-webhook-signature",
				"webhook_secret_key": f.WebhookSecret,
			}
		}
	} else if f.HMACSecret != "" {
		algo := "sha256"
		if f.HMACAlgo != "" {
			algo = strings.ToLower(f.HMACAlgo)
		}
		config["auth"] = map[string]interface{}{
			"algorithm":          algo,
			"encoding":           "hex",
			"header_key":         "x-webhook-signature",
			"webhook_secret_key": f.HMACSecret,
		}
	} else if f.APIKey != "" {
		config["auth"] = map[string]interface{}{
			"header_key": "x-api-key",
			"api_key":    f.APIKey,
		}
	} else if f.BasicAuthUser != "" || f.BasicAuthPass != "" {
		config["auth"] = map[string]interface{}{
			"username": f.BasicAuthUser,
			"password": f.BasicAuthPass,
		}
	}

	if f.AllowedHTTPMethods != "" {
		methods := strings.Split(f.AllowedHTTPMethods, ",")
		validMethods := []string{}
		allowed := map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}
		for _, method := range methods {
			method = strings.TrimSpace(strings.ToUpper(method))
			if !allowed[method] {
				return nil, fmt.Errorf("invalid HTTP method %q in %s (allowed: GET, POST, PUT, PATCH, DELETE)", method, flagRef(flagPrefix, "allowed-http-methods"))
			}
			validMethods = append(validMethods, method)
		}
		config["allowed_http_methods"] = validMethods
	}
	if f.CustomResponseType != "" || f.CustomResponseBody != "" {
		if f.CustomResponseType == "" {
			return nil, fmt.Errorf("%s is required when using %s", flagRef(flagPrefix, "custom-response-content-type"), flagRef(flagPrefix, "custom-response-body"))
		}
		if f.CustomResponseBody == "" {
			return nil, fmt.Errorf("%s is required when using %s", flagRef(flagPrefix, "custom-response-body"), flagRef(flagPrefix, "custom-response-content-type"))
		}
		validTypes := map[string]bool{"json": true, "text": true, "xml": true}
		contentType := strings.ToLower(f.CustomResponseType)
		if !validTypes[contentType] {
			return nil, fmt.Errorf("invalid content type %q in %s (allowed: json, text, xml)", f.CustomResponseType, flagRef(flagPrefix, "custom-response-content-type"))
		}
		if len(f.CustomResponseBody) > 1000 {
			return nil, fmt.Errorf("%s exceeds maximum length of 1000 characters (got %d)", flagRef(flagPrefix, "custom-response-body"), len(f.CustomResponseBody))
		}
		config["custom_response"] = map[string]interface{}{
			"content_type": contentType,
			"body":         f.CustomResponseBody,
		}
	}
	return config, nil
}

// ensureSourceConfigAuthTypeForHTTP sets config.auth_type when source type is HTTP and config
// has auth. The connection API requires auth_type in config for HTTP sources. Values: API_KEY,
// BASIC_AUTH, HMAC. No-op if auth_type already set or source type is not HTTP.
func ensureSourceConfigAuthTypeForHTTP(config map[string]interface{}, sourceType string) {
	if config == nil || strings.ToUpper(strings.TrimSpace(sourceType)) != "HTTP" {
		return
	}
	if _, hasAuth := config["auth"]; !hasAuth {
		return
	}
	if _, hasType := config["auth_type"]; hasType {
		return
	}
	auth, _ := config["auth"].(map[string]interface{})
	if auth == nil {
		return
	}
	if _, ok := auth["api_key"]; ok {
		config["auth_type"] = "API_KEY"
		return
	}
	if _, ok := auth["username"]; ok {
		config["auth_type"] = "BASIC_AUTH"
		return
	}
	if _, ok := auth["webhook_secret_key"]; ok {
		config["auth_type"] = "HMAC"
	}
}

// normalizeSourceConfigAuth converts legacy flat auth keys (webhook_secret, api_key, etc.)
// into the API shape: config.auth only (no auth_type; type is source-level, auth shape depends on it).
// Idempotent if auth already set.
func normalizeSourceConfigAuth(config map[string]interface{}, sourceType string) {
	if config == nil || config["auth"] != nil {
		return
	}
	sourceTypeUpper := strings.ToUpper(strings.TrimSpace(sourceType))
	if v, ok := config["webhook_secret"].(string); ok && v != "" {
		if sourceTypeUpper == "STRIPE" {
			config["auth"] = map[string]interface{}{"webhook_secret_key": v}
		} else {
			config["auth"] = map[string]interface{}{
				"algorithm": "sha256", "encoding": "hex",
				"header_key": "x-webhook-signature", "webhook_secret_key": v,
			}
		}
		delete(config, "webhook_secret")
		return
	}
	if v, ok := config["api_key"].(string); ok && v != "" {
		config["auth"] = map[string]interface{}{"header_key": "x-api-key", "api_key": v}
		delete(config, "api_key")
		return
	}
	if m, ok := config["basic_auth"].(map[string]interface{}); ok {
		u, _ := m["username"].(string)
		p, _ := m["password"].(string)
		if u != "" || p != "" {
			config["auth"] = map[string]interface{}{"username": u, "password": p}
			delete(config, "basic_auth")
		}
		return
	}
	if m, ok := config["hmac"].(map[string]interface{}); ok {
		secret, _ := m["secret"].(string)
		if secret != "" {
			algo := "sha256"
			if a, _ := m["algorithm"].(string); a != "" {
				algo = strings.ToLower(a)
			}
			config["auth"] = map[string]interface{}{
				"algorithm": algo, "encoding": "hex",
				"header_key": "x-webhook-signature", "webhook_secret_key": secret,
			}
			delete(config, "hmac")
		}
	}
}

// buildSourceConfigFromFlags parses source config from --config/--config-file (JSON)
// or from individual flags. sourceType (e.g. WEBHOOK, STRIPE) is used for correct auth shape.
// When configStr or configFile is set, that takes precedence.
// Used by source create, upsert, and update. Returns (nil, nil) when nothing is set.
// Normalizes legacy flat auth keys to auth_type + auth so the API accepts the payload.
func buildSourceConfigFromFlags(configStr, configFile string, individual *sourceConfigFlags, sourceType string) (map[string]interface{}, error) {
	if configStr != "" {
		var out map[string]interface{}
		if err := json.Unmarshal([]byte(configStr), &out); err != nil {
			return nil, fmt.Errorf("invalid JSON in --config: %w", err)
		}
		normalizeSourceConfigAuth(out, sourceType)
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
		normalizeSourceConfigAuth(out, sourceType)
		return out, nil
	}
	return buildSourceConfigFromIndividualFlags(individual, "", sourceType)
}

// sourceAuthFlags holds the auth-related flag values for spec-based validation.
// Used by source create/upsert (unprefixed) and connection create (--source-* prefixed).
type sourceAuthFlags struct {
	WebhookSecret string
	APIKey        string
	BasicAuthUser string
	BasicAuthPass string
	HMACSecret    string
}

// optionalAuthSourceTypes are source types where authentication can be turned on or off
// but is not required. We do not reject these when auth flags are missing.
var optionalAuthSourceTypes = map[string]bool{"STRIPE": true}

// validateSourceAuthFromSpec uses the cached OpenAPI spec (FetchSourceTypes) to validate
// that the given source type has the required auth flags set. Used by source create/upsert
// (flagPrefix "") and connection create (flagPrefix "source-"). If configSet, skip validation.
// Types in optionalAuthSourceTypes are not required to have auth set. If FetchSourceTypes
// fails or the type is unknown, returns nil so the API can validate.
func validateSourceAuthFromSpec(sourceType string, configSet bool, auth sourceAuthFlags, flagPrefix string) error {
	if sourceType == "" || configSet {
		return nil
	}
	if optionalAuthSourceTypes[strings.ToUpper(sourceType)] {
		return nil
	}
	sourceTypes, err := sources.FetchSourceTypes()
	if err != nil {
		fmt.Printf("Warning: could not fetch source types for validation: %v\n", err)
		return nil
	}
	st, ok := sourceTypes[strings.ToUpper(sourceType)]
	if !ok {
		return nil
	}
	pre := "--" + flagPrefix
	switch st.AuthScheme {
	case "webhook_secret":
		if auth.WebhookSecret == "" {
			return fmt.Errorf("%swebhook-secret is required for source type %s", pre, sourceType)
		}
	case "api_key":
		if auth.APIKey == "" {
			return fmt.Errorf("%sapi-key is required for source type %s", pre, sourceType)
		}
	case "basic_auth":
		if auth.BasicAuthUser == "" || auth.BasicAuthPass == "" {
			return fmt.Errorf("%sbasic-auth-user and %sbasic-auth-pass are required for source type %s", pre, pre, sourceType)
		}
	case "hmac":
		if auth.HMACSecret == "" {
			return fmt.Errorf("%shmac-secret is required for source type %s", pre, sourceType)
		}
	}
	return nil
}

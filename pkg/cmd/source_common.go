package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/hookdeck/hookdeck-cli/pkg/cmd/sources"
)

// sourceConfigFlags holds individual source config flags (no "source-" prefix).
// Used by source create, upsert, and update. Same semantics as connection's
// --source-* flags; when both --config/--config-file and individual flags are
// set, --config/--config-file take precedence.
type sourceConfigFlags struct {
	WebhookSecret      string
	APIKey             string
	BasicAuthUser      string
	BasicAuthPass      string
	HMACSecret         string
	HMACAlgo           string
	AllowedHTTPMethods string
	CustomResponseBody string
	CustomResponseType string
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
// contents depend on that type (per OpenAPI SourceTypeConfig oneOf). Types offering a choice of
// method (WEBHOOK, HTTP, ...) also need config.auth_type; ensureSourceConfigAuthType adds it.
// Shared by source create/upsert/update (prefix "") and connection create/upsert (prefix "source-").
// flagPrefix is used only in error messages so connection errors mention --source-*.
func buildSourceConfigFromIndividualFlags(f *sourceConfigFlags, flagPrefix, sourceType string) (map[string]interface{}, error) {
	if f == nil || !f.hasAny() {
		return nil, nil
	}
	config := make(map[string]interface{})

	// Auth: config.auth shape depends on source type. auth_type is added at the end for the
	// types that require it (see ensureSourceConfigAuthType).
	auth, authType, err := sourceAuthFromFlags(f, flagPrefix, sourceType)
	if err != nil {
		return nil, err
	}
	if auth != nil {
		config["auth"] = auth
		if authType != "" {
			config["auth_type"] = authType
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
	ensureSourceConfigAuthType(config, sourceType)
	return config, nil
}

// apiDefaultSourceType is the type the API applies when a source create or upsert omits it
// (the create schema defaults type to WEBHOOK), so that is the type its config is validated
// against. Building config for an omitted type must use the same one.
const apiDefaultSourceType = "WEBHOOK"

// sourceTypeOrAPIDefault returns sourceType, or apiDefaultSourceType when it is empty.
// Only for create/upsert paths: an update of an existing source keeps the stored type.
func sourceTypeOrAPIDefault(sourceType string) string {
	if strings.TrimSpace(sourceType) == "" {
		return apiDefaultSourceType
	}
	return sourceType
}

// loadSourceAuthMethods returns each source type's auth methods from the cached OpenAPI spec.
// A variable so tests serve a fixture instead of downloading the spec.
var loadSourceAuthMethods = sources.FetchSourceAuthMethods

// builtinGenericAuthMethods are the WEBHOOK and HTTP auth methods as the API defines them,
// used when the spec cannot be fetched so the auth flags keep working offline for the two
// generic types. Other types need the spec.
var builtinGenericAuthMethods = []sources.AuthMethod{
	{Name: "HMAC", Properties: []string{"algorithm", "encoding", "header_key", "webhook_secret_key"}, Required: []string{"algorithm", "encoding", "header_key", "webhook_secret_key"}},
	{Name: "BASIC_AUTH", Properties: []string{"password", "username"}, Required: []string{"username", "password"}},
	{Name: "API_KEY", Properties: []string{"api_key", "header_key"}, Required: []string{"header_key", "api_key"}},
	{Name: "STANDARD_WEBHOOKS", Properties: []string{"webhook_secret_key"}, Required: []string{"webhook_secret_key"}},
}

// ensureSourceConfigAuthType sets config.auth_type when config has auth but no auth_type and
// the source type offers a choice of verification method.
//
// For those types (WEBHOOK, HTTP, ORDINAL, PROMETHEUS_ALERTMANAGER and others, as listed in the
// OpenAPI spec) the API validates config.auth against the method named in config.auth_type,
// and rejects config.auth without it ("config.auth is not allowed"). Types with one fixed
// method (STRIPE, GITHUB, ...) take no auth_type, so nothing is added for them.
//
// The method is the one whose fields fit the auth keys (sources.InferAuthType). For example,
// on WEBHOOK: api_key + header_key -> API_KEY, username + password -> BASIC_AUTH, a secret with
// algorithm/encoding/header_key -> HMAC, and a bare webhook_secret_key -> STANDARD_WEBHOOKS
// (HMAC requires the other fields, so Standard Webhooks is the only method that accepts it).
// When no single method fits, auth_type is left unset and the API reports the problem.
func ensureSourceConfigAuthType(config map[string]interface{}, sourceType string) {
	if config == nil || strings.TrimSpace(sourceType) == "" {
		return
	}
	if _, hasType := config["auth_type"]; hasType {
		return
	}
	auth, _ := config["auth"].(map[string]interface{})
	if len(auth) == 0 {
		return
	}

	var methods []sources.AuthMethod
	if all, err := loadSourceAuthMethods(); err == nil {
		methods = sources.AuthMethodsFor(all, sourceType)
	} else {
		upper := strings.ToUpper(strings.TrimSpace(sourceType))
		if upper == "WEBHOOK" || upper == "HTTP" {
			methods = builtinGenericAuthMethods
		}
		log.WithError(err).Debug("could not load source auth methods from the OpenAPI spec; config.auth_type inferred only for WEBHOOK and HTTP")
	}

	keys := make([]string, 0, len(auth))
	for k := range auth {
		keys = append(keys, k)
	}
	if authType := sources.InferAuthType(methods, keys); authType != "" {
		config["auth_type"] = authType
	}
}

// normalizeSourceConfigAuth converts the legacy top-level auth keys in a source config
// (webhook_secret, api_key, basic_auth, hmac) into config.auth, and config.auth_type where the
// type offers a choice of method. It builds them exactly as the matching flags do
// (webhook_secret as --webhook-secret, api_key as --api-key, hmac as --hmac-secret, basic_auth as
// the basic auth flags), so the same per-type shapes and refusals apply. Does nothing when
// config.auth is already set. An explicit config.auth_type is kept.
func normalizeSourceConfigAuth(config map[string]interface{}, sourceType string) error {
	if config == nil || config["auth"] != nil {
		return nil
	}
	var flags *sourceConfigFlags
	var legacyKey string
	if v, ok := config["webhook_secret"].(string); ok && v != "" {
		flags, legacyKey = &sourceConfigFlags{WebhookSecret: v}, "webhook_secret"
	} else if v, ok := config["api_key"].(string); ok && v != "" {
		flags, legacyKey = &sourceConfigFlags{APIKey: v}, "api_key"
	} else if m, ok := config["basic_auth"].(map[string]interface{}); ok {
		u, _ := m["username"].(string)
		p, _ := m["password"].(string)
		if u != "" || p != "" {
			flags, legacyKey = &sourceConfigFlags{BasicAuthUser: u, BasicAuthPass: p}, "basic_auth"
		}
	} else if m, ok := config["hmac"].(map[string]interface{}); ok {
		secret, _ := m["secret"].(string)
		if secret != "" {
			algo, _ := m["algorithm"].(string)
			flags, legacyKey = &sourceConfigFlags{HMACSecret: secret, HMACAlgo: algo}, "hmac"
		}
	}
	if flags == nil {
		return nil
	}
	auth, authType, err := sourceAuthFromFlags(flags, "", sourceType)
	if err != nil {
		return fmt.Errorf("config key %q: %w", legacyKey, err)
	}
	config["auth"] = auth
	if _, has := config["auth_type"]; !has && authType != "" {
		config["auth_type"] = authType
	}
	delete(config, legacyKey)
	return nil
}

// buildSourceConfigFromFlags parses source config from --config/--config-file (JSON)
// or from individual flags. sourceType (e.g. WEBHOOK, STRIPE) is used for correct auth shape.
// When configStr or configFile is set, that takes precedence.
// Used by source create, upsert, and update. Returns (nil, nil) when nothing is set.
// Normalizes legacy flat auth keys to config.auth and, for types offering a choice of auth
// method, sets config.auth_type so the API accepts the payload.
func buildSourceConfigFromFlags(configStr, configFile string, individual *sourceConfigFlags, sourceType string) (map[string]interface{}, error) {
	if configStr != "" {
		var out map[string]interface{}
		if err := json.Unmarshal([]byte(configStr), &out); err != nil {
			return nil, fmt.Errorf("invalid JSON in --config: %w", err)
		}
		if err := normalizeSourceConfigAuth(out, sourceType); err != nil {
			return nil, err
		}
		ensureSourceConfigAuthType(out, sourceType)
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
		if err := normalizeSourceConfigAuth(out, sourceType); err != nil {
			return nil, err
		}
		ensureSourceConfigAuthType(out, sourceType)
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

// optionalAuthSourceTypes are source types the spec reads as auth-required although
// verification is optional. The spec marks a type's config required when any feature is
// required (see sources.ParseSourceTypes), and ONFLEET's required feature is its handshake.
var optionalAuthSourceTypes = map[string]bool{"ONFLEET": true}

// loadSourceTypes returns each source type's auth rules from the cached OpenAPI spec.
// A variable so tests serve a fixture instead of downloading the spec.
var loadSourceTypes = sources.FetchSourceTypes

// authFlagOptions are the auth flags a validation error can point to, each with sample values
// so sourceAuthFromFlags reports the config.auth keys it sets. --hmac-secret is not listed: it
// sets a secret like --webhook-secret does, and the message points at the plainer flag.
var authFlagOptions = []struct {
	names []string
	flags sourceConfigFlags
}{
	{[]string{"webhook-secret"}, sourceConfigFlags{WebhookSecret: "-"}},
	{[]string{"api-key"}, sourceConfigFlags{APIKey: "-"}},
	{[]string{"basic-auth-user", "basic-auth-pass"}, sourceConfigFlags{BasicAuthUser: "-", BasicAuthPass: "-"}},
}

// validateSourceAuthFromSpec checks the auth flags against the source type's config.auth
// fields in the cached OpenAPI spec (FetchSourceTypes). Used by source create/upsert
// (flagPrefix "") and connection create (flagPrefix "source-").
//
// It rejects two cases, both of which the API would reject:
//   - no auth flag, for a type that requires auth (AIRCALL, ZOOM, ...), and
//   - an auth flag that does not set a field the type's auth requires (--api-key for STRIPE),
//     since the API strips the keys it does not take and then finds the required one missing.
//
// The error names the flag that sets the fields, or --config when no flag does (ALIPAY's
// public_key, for example). Types offering a choice of method (WEBHOOK, HTTP, ...) are left to
// the API. If configSet, the user wrote config.auth themselves and nothing is checked. If the
// spec cannot be fetched or the type is not in it, returns nil so the API can validate.
func validateSourceAuthFromSpec(sourceType string, configSet bool, auth sourceAuthFlags, flagPrefix string) error {
	if sourceType == "" || configSet {
		return nil
	}
	sourceTypes, err := loadSourceTypes()
	if err != nil {
		// stderr, not stdout. This is advisory — validation is skipped and the
		// API still checks — but on stdout it lands ahead of the command's own
		// output, and `--output json` stops being parseable:
		//
		//   failed to unmarshal JSON output: invalid character 'W' looking for
		//   beginning of value
		//
		// The spec fetch fails only intermittently, so this broke scripts at
		// random rather than consistently.
		fmt.Fprintf(os.Stderr, "Warning: could not fetch source types for validation: %v\n", err)
		return nil
	}
	st, ok := sources.SourceTypeFor(sourceTypes, sourceType)
	if !ok {
		return nil
	}
	pre := "--" + flagPrefix

	given, _, err := sourceAuthFromFlags(&sourceConfigFlags{
		WebhookSecret: auth.WebhookSecret,
		APIKey:        auth.APIKey,
		BasicAuthUser: auth.BasicAuthUser,
		BasicAuthPass: auth.BasicAuthPass,
		HMACSecret:    auth.HMACSecret,
	}, flagPrefix, sourceType)
	if err != nil {
		return err
	}

	if given == nil {
		if !st.AuthRequired || optionalAuthSourceTypes[st.Name] {
			return nil
		}
		// A required auth whose fields are all optional still needs one of them to say anything.
		need := st.RequiredAuthFields
		if len(need) == 0 {
			need = st.AuthFields
		}
		if flags := authFlagsSetting(need, sourceType, pre); flags != nil {
			verb := "is"
			if len(flags) > 1 {
				verb = "are"
			}
			return fmt.Errorf("%s %s required for source type %s", strings.Join(flags, " and "), verb, sourceType)
		}
		return fmt.Errorf("source type %s requires config.auth %s, which no flag sets: %s",
			sourceType, strings.Join(need, ", "), configHint(need, pre))
	}

	var missing []string
	for _, field := range st.RequiredAuthFields {
		if _, ok := given[field]; !ok {
			missing = append(missing, field)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	used := pre + givenAuthFlag(auth)
	if flags := authFlagsSetting(st.RequiredAuthFields, sourceType, pre); flags != nil {
		return fmt.Errorf("%s does not set config.auth %s, which source type %s requires: use %s",
			used, strings.Join(missing, ", "), sourceType, strings.Join(flags, " and "))
	}
	return fmt.Errorf("%s does not set config.auth %s, which source type %s requires: %s",
		used, strings.Join(missing, ", "), sourceType, configHint(st.RequiredAuthFields, pre))
}

// authFlagsSetting returns the flags (with "--" and prefix) whose config.auth carries every one
// of fields, or nil when no flag does.
func authFlagsSetting(fields []string, sourceType, pre string) []string {
	for _, opt := range authFlagOptions {
		keys, _, err := sourceAuthFromFlags(&opt.flags, "", sourceType)
		if err != nil {
			continue
		}
		covers := true
		for _, field := range fields {
			if _, ok := keys[field]; !ok {
				covers = false
				break
			}
		}
		if covers {
			out := make([]string, len(opt.names))
			for i, name := range opt.names {
				out[i] = pre + name
			}
			return out
		}
	}
	return nil
}

// givenAuthFlag names the auth flag sourceAuthFromFlags used, in its precedence order.
func givenAuthFlag(auth sourceAuthFlags) string {
	switch {
	case auth.WebhookSecret != "":
		return "webhook-secret"
	case auth.HMACSecret != "":
		return "hmac-secret"
	case auth.APIKey != "":
		return "api-key"
	case auth.BasicAuthUser != "":
		return "basic-auth-user"
	default:
		return "basic-auth-pass"
	}
}

// configHint shows how to pass config.auth fields no flag sets.
func configHint(fields []string, pre string) string {
	parts := make([]string, len(fields))
	for i, field := range fields {
		parts[i] = fmt.Sprintf("%q:\"...\"", field)
	}
	return fmt.Sprintf("pass them with %sconfig '{\"auth\":{%s}}' or %sconfig-file",
		pre, strings.Join(parts, ","), pre)
}

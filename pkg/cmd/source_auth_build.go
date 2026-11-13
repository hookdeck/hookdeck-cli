package cmd

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/hookdeck/hookdeck-cli/pkg/cmd/sources"
)

// sourceAuthMethodSet is what the API accepts in config.auth for one source type.
type sourceAuthMethodSet struct {
	// methods are the verification methods the type accepts. A type with one fixed method has
	// one entry with no name.
	methods []sources.AuthMethod
	// choice is true when the type offers a choice of method, so config.auth_type must name one.
	choice bool
	// known is false when the spec could not be read or does not list the type.
	known bool
}

// isGenericSourceType reports whether sourceType is WEBHOOK or HTTP, the two types that take any
// signing scheme. Their methods are fixed by the API, so the CLI does not need the spec for them.
func isGenericSourceType(sourceType string) bool {
	switch strings.ToUpper(strings.TrimSpace(sourceType)) {
	case "WEBHOOK", "HTTP":
		return true
	}
	return false
}

// resolveSourceAuthMethods reads the type's auth methods from the cached OpenAPI spec.
func resolveSourceAuthMethods(sourceType string) sourceAuthMethodSet {
	if isGenericSourceType(sourceType) {
		return sourceAuthMethodSet{methods: builtinGenericAuthMethods, choice: true, known: true}
	}
	if strings.TrimSpace(sourceType) == "" {
		return sourceAuthMethodSet{}
	}
	if all, err := loadSourceAuthMethods(); err != nil {
		log.WithError(err).Debug("could not load source auth methods from the OpenAPI spec")
	} else if m := sources.AuthMethodsFor(all, sourceType); m != nil {
		return sourceAuthMethodSet{methods: m, choice: true, known: true}
	}
	types, err := loadSourceTypes()
	if err != nil {
		log.WithError(err).Debug("could not load source types from the OpenAPI spec")
		return sourceAuthMethodSet{}
	}
	st, ok := sources.SourceTypeFor(types, sourceType)
	if !ok || len(st.AuthFields) == 0 {
		return sourceAuthMethodSet{}
	}
	return sourceAuthMethodSet{
		methods: []sources.AuthMethod{{Properties: st.AuthFields, Required: st.RequiredAuthFields}},
		known:   true,
	}
}

func (m sourceAuthMethodSet) accepting(fields ...string) []sources.AuthMethod {
	var out []sources.AuthMethod
	for _, method := range m.methods {
		ok := true
		for _, f := range fields {
			if !hasField(method.Properties, f) {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, method)
		}
	}
	return out
}

func hasField(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func (m sourceAuthMethodSet) names() string {
	names := make([]string, 0, len(m.methods))
	for _, method := range m.methods {
		names = append(names, method.Name)
	}
	return strings.Join(names, ", ")
}

// sourceAuthFromFlags builds config.auth, and config.auth_type for a type offering a choice of
// method, from the auth flags. It returns a nil auth when no auth flag is set. Only the first
// set flag counts, in this order: webhook secret, HMAC secret, API key, basic auth.
//
// The fields sent come from the source type's own auth methods in the OpenAPI spec, not from one
// shape for every type. That matters because the API merges a source's config.auth over the
// provider's own defaults when it verifies a request (a user-supplied header_key replaces
// VAPI's x-signature, for example), and strips only the keys a method does not list. So:
//
//   - --webhook-secret sends webhook_secret_key, plus algorithm, encoding and header_key only
//     where the method requires them (WEBHOOK and HTTP HMAC: sha256, hex, x-webhook-signature).
//     Everywhere else the provider's defaults apply.
//   - --hmac-secret sends that generic HMAC shape wherever a method takes algorithm, filtered
//     to the keys the method lists; otherwise it behaves as --webhook-secret.
//   - --api-key sends api_key. A header_key the type requires has no flag: WEBHOOK and HTTP get
//     x-api-key, and every other type is refused with the --config form to use.
//
// When the spec cannot be read, WEBHOOK and HTTP still work; other types get only the secret (or
// key), which is safe because the provider's defaults then apply.
func sourceAuthFromFlags(f *sourceConfigFlags, flagPrefix, sourceType string) (map[string]interface{}, string, error) {
	pre := "--" + flagPrefix
	set := resolveSourceAuthMethods(sourceType)

	switch {
	case f.WebhookSecret != "":
		return secretAuth(set, f.WebhookSecret, false, f, pre, sourceType)
	case f.HMACSecret != "":
		return secretAuth(set, f.HMACSecret, true, f, pre, sourceType)
	case f.APIKey != "":
		return apiKeyAuth(set, f.APIKey, pre, sourceType)
	case f.BasicAuthUser != "" || f.BasicAuthPass != "":
		auth := map[string]interface{}{"username": f.BasicAuthUser, "password": f.BasicAuthPass}
		if !set.choice {
			return auth, "", nil
		}
		ms := set.accepting("username", "password")
		if len(ms) == 0 {
			return nil, "", unsupportedAuthFlag(set, pre+"basic-auth-user and "+pre+"basic-auth-pass", pre, sourceType)
		}
		return auth, ms[0].Name, nil
	}
	return nil, "", nil
}

func secretAuth(set sourceAuthMethodSet, secret string, explicitHMAC bool, f *sourceConfigFlags, pre, sourceType string) (map[string]interface{}, string, error) {
	flagName := pre + "webhook-secret"
	if explicitHMAC {
		flagName = pre + "hmac-secret"
	}
	auth := map[string]interface{}{"webhook_secret_key": secret}
	if !set.known {
		return auth, "", nil
	}

	algo := "sha256"
	if explicitHMAC && f.HMACAlgo != "" {
		algo = strings.ToLower(f.HMACAlgo)
	}
	generic := map[string]interface{}{"algorithm": algo, "encoding": "hex", "header_key": "x-webhook-signature"}

	var method *sources.AuthMethod
	if explicitHMAC {
		if ms := set.accepting("webhook_secret_key", "algorithm"); len(ms) > 0 {
			method = &ms[0]
			for k, v := range generic {
				if hasField(method.Properties, k) {
					auth[k] = v
				}
			}
		}
	}
	if method == nil {
		ms := set.accepting("webhook_secret_key")
		if len(ms) == 0 {
			if !set.choice {
				// One fixed method with no secret field: the required-field check names the flag to use.
				return auth, "", nil
			}
			return nil, "", unsupportedAuthFlag(set, flagName, pre, sourceType)
		}
		method = &ms[0]
		if explicitHMAC && f.HMACAlgo != "" {
			return nil, "", fmt.Errorf("%s does not apply to source type %s: its %s method takes no algorithm",
				pre+"hmac-algo", sourceType, methodLabel(*method))
		}
	}
	// Whatever the method requires beyond the secret and a flag can supply gets the generic value.
	for _, r := range method.Required {
		if _, done := auth[r]; done {
			continue
		}
		v, ok := generic[r]
		if !ok {
			return nil, "", fmt.Errorf("%s does not set config.auth %s, which source type %s requires: %s",
				flagName, r, sourceType, configHint([]string{r, "webhook_secret_key"}, pre))
		}
		auth[r] = v
	}
	return auth, authTypeFor(set, *method), nil
}

func apiKeyAuth(set sourceAuthMethodSet, key, pre, sourceType string) (map[string]interface{}, string, error) {
	auth := map[string]interface{}{"api_key": key}
	if !set.known {
		return auth, "", nil
	}
	ms := set.accepting("api_key")
	if len(ms) == 0 {
		if !set.choice {
			return auth, "", nil
		}
		return nil, "", unsupportedAuthFlag(set, pre+"api-key", pre, sourceType)
	}
	method := ms[0]
	if hasField(method.Properties, "header_key") && (len(method.Required) == 0 || hasField(method.Required, "header_key")) {
		if !isGenericSourceType(sourceType) {
			// The header name is part of the provider's contract, and no flag sets it.
			return nil, "", fmt.Errorf("%sapi-key does not set config.auth header_key, which source type %s requires: %s",
				pre, sourceType, configHint([]string{"header_key", "api_key"}, pre))
		}
		auth["header_key"] = "x-api-key"
	}
	return auth, authTypeFor(set, method), nil
}

func authTypeFor(set sourceAuthMethodSet, method sources.AuthMethod) string {
	if set.choice {
		return method.Name
	}
	return ""
}

func methodLabel(m sources.AuthMethod) string {
	if m.Name == "" {
		return "auth"
	}
	return m.Name
}

func unsupportedAuthFlag(set sourceAuthMethodSet, flags, pre, sourceType string) error {
	return fmt.Errorf("%s does not apply to source type %s, whose auth methods are %s; set config.auth and auth_type with %sconfig instead",
		flags, sourceType, set.names(), pre)
}

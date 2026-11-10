package sources

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// authMethodsCacheFileName caches the parsed auth methods, keyed to the API version like
// cacheFileName so an upgrade never reads methods parsed from an older spec.
var authMethodsCacheFileName = "hookdeck_source_auth_methods" + strings.ReplaceAll(hookdeck.APIPathPrefix, "/", "_") + ".json"

// AuthMethod is one verification method a source type's config accepts: the value to send
// as config.auth_type, and the fields config.auth may carry for it.
type AuthMethod struct {
	Name       string   `json:"name"`
	Properties []string `json:"properties"`
	Required   []string `json:"required"`
}

// FetchSourceAuthMethods returns, for each source type whose config takes config.auth_type
// (a type offering a choice of verification method), the methods it accepts in spec order.
// Types with a single fixed method take no auth_type and are absent from the map.
//
// It downloads the OpenAPI spec and caches the parsed result for cacheTTL.
func FetchSourceAuthMethods() (map[string][]AuthMethod, error) {
	cachePath := filepath.Join(os.TempDir(), authMethodsCacheFileName)

	if info, err := os.Stat(cachePath); err == nil && time.Since(info.ModTime()) < cacheTTL {
		if data, err := os.ReadFile(cachePath); err == nil {
			var cached map[string][]AuthMethod
			if json.Unmarshal(data, &cached) == nil && len(cached) > 0 {
				return cached, nil
			}
		}
	}

	resp, err := http.Get(openapiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download OpenAPI spec: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download OpenAPI spec: received status code %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read OpenAPI spec body: %w", err)
	}

	methods, err := ParseSourceAuthMethods(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}
	if len(methods) == 0 {
		// Do not cache an empty result: it would hide a spec-shape change for a day.
		return nil, fmt.Errorf("failed to parse OpenAPI spec: no source auth methods found")
	}

	if data, err := json.Marshal(methods); err == nil {
		_ = os.WriteFile(cachePath, data, 0o600)
	}
	return methods, nil
}

// specSchema is the subset of an OpenAPI schema object this parser reads.
type specSchema struct {
	Ref        string                `json:"$ref"`
	Properties map[string]specSchema `json:"properties"`
	Required   []string              `json:"required"`
	Enum       []string              `json:"enum"`
	OneOf      []specSchema          `json:"oneOf"`
	DocsType   string                `json:"x-docs-type"`
}

const sourceTypeConfigPrefix = "SourceTypeConfig"

// ParseSourceAuthMethods extracts each source type's auth methods from the OpenAPI spec.
//
// The spec has one SourceTypeConfig<TYPE> schema per source type. A type offering a choice of
// method has an auth_type enum, and its auth property refers to a oneOf with one branch per
// method; each branch names its method in x-docs-type and lists its fields.
//
// WEBHOOK's enum also lists every provider type as a legacy method (for configs created by
// old API versions). Those are left out: a method named after another source type is never
// what a WEBHOOK auth flag means, and keeping them would make every shape ambiguous.
func ParseSourceAuthMethods(specData []byte) (map[string][]AuthMethod, error) {
	var spec struct {
		Components struct {
			Schemas map[string]specSchema `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(specData, &spec); err != nil {
		return nil, err
	}
	schemas := spec.Components.Schemas
	resolve := func(s specSchema) specSchema {
		if s.Ref == "" {
			return s
		}
		return schemas[s.Ref[strings.LastIndex(s.Ref, "/")+1:]]
	}

	sourceTypes := map[string]bool{}
	for name := range schemas {
		if strings.HasPrefix(name, sourceTypeConfigPrefix) {
			sourceTypes[normalizeTypeName(strings.TrimPrefix(name, sourceTypeConfigPrefix))] = true
		}
	}

	out := map[string][]AuthMethod{}
	for name, schema := range schemas {
		if !strings.HasPrefix(name, sourceTypeConfigPrefix) {
			continue
		}
		sourceType := strings.TrimPrefix(name, sourceTypeConfigPrefix)
		authType, ok := schema.Properties["auth_type"]
		if !ok || len(authType.Enum) == 0 {
			continue
		}
		allowed := map[string]bool{}
		for _, v := range authType.Enum {
			allowed[v] = true
		}

		var methods []AuthMethod
		for _, branch := range resolve(schema.Properties["auth"]).OneOf {
			branch = resolve(branch)
			method := branch.DocsType
			if method == "" || !allowed[method] {
				continue // the hidden "Empty" branch, or a method not in the enum
			}
			if normalizeTypeName(method) != normalizeTypeName(sourceType) && sourceTypes[normalizeTypeName(method)] {
				continue // legacy provider method on WEBHOOK
			}
			props := make([]string, 0, len(branch.Properties))
			for p := range branch.Properties {
				props = append(props, p)
			}
			sort.Strings(props)
			methods = append(methods, AuthMethod{Name: method, Properties: props, Required: branch.Required})
		}
		if len(methods) > 0 {
			out[sourceType] = methods
		}
	}
	return out, nil
}

// normalizeTypeName maps a source type name to the form used in spec schema names, which drop
// characters such as "-" (type PROPERTY-FINDER has schema SourceTypeConfigPROPERTYFINDER).
func normalizeTypeName(name string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(strings.TrimSpace(name)) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// AuthMethodsFor returns the auth methods for sourceType (any case or spelling the API
// accepts), or nil when the type takes no auth_type.
func AuthMethodsFor(methods map[string][]AuthMethod, sourceType string) []AuthMethod {
	want := normalizeTypeName(sourceType)
	for name, m := range methods {
		if normalizeTypeName(name) == want {
			return m
		}
	}
	return nil
}

// InferAuthType picks the auth method whose fields fit the given config.auth keys, or ""
// when none or more than one fits equally well.
//
// A method fits when every key is one of its fields and every field it marks required is
// present. The spec lists required fields for some types only (WEBHOOK's HMAC lists them,
// HTTP's does not), so among fitting methods the one with the fewest unused fields wins: a
// bare webhook_secret_key fits HMAC and STANDARD_WEBHOOKS on HTTP, and only
// STANDARD_WEBHOOKS uses nothing else - which is also the only one the API accepts for it.
// A tie is left unresolved rather than guessed.
func InferAuthType(methods []AuthMethod, authKeys []string) string {
	if len(authKeys) == 0 {
		return ""
	}
	keys := map[string]bool{}
	for _, k := range authKeys {
		keys[k] = true
	}
	best, bestExtra, tied := "", -1, false
	for _, m := range methods {
		props := map[string]bool{}
		for _, p := range m.Properties {
			props[p] = true
		}
		fits := true
		for k := range keys {
			if !props[k] {
				fits = false
				break
			}
		}
		for _, r := range m.Required {
			if !keys[r] {
				fits = false
				break
			}
		}
		if !fits {
			continue
		}
		extra := len(props) - len(keys)
		switch {
		case bestExtra == -1 || extra < bestExtra:
			best, bestExtra, tied = m.Name, extra, false
		case extra == bestExtra:
			tied = true
		}
	}
	if tied {
		return ""
	}
	return best
}

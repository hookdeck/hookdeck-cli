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

var (
	// Both derived from the one version constant, so they cannot drift apart.
	// They did: the URL moved to 2026-09-01 while the cache name stayed put, so
	// upgrading within the 24 hour TTL served source types parsed from the
	// previous spec - stale auth schemes and required fields, used to validate
	// `source create`.
	//
	// The "_v2" marks the cached shape. Versions that read SourceCreateRequest wrote an empty
	// map under the unversioned name, and a different struct; a new name means neither is read.
	openapiURL    = "https://api.hookdeck.com" + hookdeck.APIPathPrefix + "/openapi"
	cacheFileName = "hookdeck_source_types_v2" + strings.ReplaceAll(hookdeck.APIPathPrefix, "/", "_") + ".json"
	cacheTTL      = 24 * time.Hour
)

// SourceType holds what the OpenAPI spec says about one source type's config.auth.
type SourceType struct {
	// Name is the type as the API spells it (PROPERTY-FINDER, not the schema's PROPERTYFINDER).
	Name string `json:"name"`
	// AuthRequired is true when a source of this type cannot be created without config.auth.
	AuthRequired bool `json:"auth_required"`
	// AuthFields are the fields config.auth takes, and RequiredAuthFields those of them it must
	// carry when sent. Both are empty for types offering a choice of method (WEBHOOK, HTTP, ...):
	// their fields depend on config.auth_type, see FetchSourceAuthMethods.
	AuthFields         []string `json:"auth_fields"`
	RequiredAuthFields []string `json:"required_auth_fields"`
}

// FetchSourceTypes downloads the OpenAPI spec, parses each source type's auth rules, and caches
// the result for cacheTTL. The map is keyed by normalized type name: look types up with
// SourceTypeFor.
//
// An empty parse is an error and is not cached: it means the spec no longer has the shape this
// parser reads, and caching it would switch validation off for a day without a word.
func FetchSourceTypes() (map[string]SourceType, error) {
	cachePath := filepath.Join(os.TempDir(), cacheFileName)

	if info, err := os.Stat(cachePath); err == nil && time.Since(info.ModTime()) < cacheTTL {
		if data, err := os.ReadFile(cachePath); err == nil {
			var cached map[string]SourceType
			if json.Unmarshal(data, &cached) == nil && len(cached) > 0 {
				return cached, nil
			}
		}
	}

	body, err := downloadOpenAPISpec()
	if err != nil {
		return nil, err
	}
	sourceTypes, err := ParseSourceTypes(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}
	if len(sourceTypes) == 0 {
		return nil, fmt.Errorf("failed to parse OpenAPI spec: no source types found")
	}

	if data, err := json.Marshal(sourceTypes); err == nil {
		_ = os.WriteFile(cachePath, data, 0o600)
	}
	return sourceTypes, nil
}

// downloadOpenAPISpec fetches the API's OpenAPI spec.
func downloadOpenAPISpec() ([]byte, error) {
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
	return body, nil
}

// ParseSourceTypes extracts each source type's auth rules from the OpenAPI spec.
//
// Every SourceTypeConfig<TYPE> schema is a source type. Its auth property is either a single
// object (a type with one fixed verification method, e.g. STRIPE) or, alongside an auth_type
// enum, a oneOf of methods (WEBHOOK, HTTP, ...).
//
// Whether auth is required is not on SourceTypeConfig<TYPE>: that schema is shared by request
// and response, so its auth is always nullable. The request-side rule shows up as
// "x-required": true on the type's entry in SourceTypeConfig.oneOf. That flag means the type's
// config is required, which the API sets when any feature is required - verification, or a
// handshake (STRAVA, MONDAY, ONFLEET, ...). So auth counts as required only when, as well:
//
//   - the type has one fixed method: for a choice of method the API always accepts
//     auth_type null with no auth, and
//   - that method has fields: a handshake-only type's auth is an empty object, nothing to send.
//
// A "required" list naming auth on SourceTypeConfig<TYPE> also counts, should the spec start
// stating it there. One handshake type still reads as auth-required (ONFLEET: handshake
// required, verification optional, an auth with a secret field); callers correct for it.
func ParseSourceTypes(specData []byte) (map[string]SourceType, error) {
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
		return schemas[refName(s.Ref)]
	}

	configRequired := map[string]bool{}
	for _, entry := range schemas[sourceTypeConfigPrefix].OneOf {
		if entry.XRequired && entry.Ref != "" {
			configRequired[refName(entry.Ref)] = true
		}
	}

	out := map[string]SourceType{}
	for schemaName, schema := range schemas {
		if !strings.HasPrefix(schemaName, sourceTypeConfigPrefix) || schemaName == sourceTypeConfigPrefix {
			continue
		}
		name := schema.DocsType
		if name == "" {
			name = strings.TrimPrefix(schemaName, sourceTypeConfigPrefix)
		}
		st := SourceType{Name: name}

		_, hasAuthType := schema.Properties["auth_type"]
		authProp, hasAuth := schema.Properties["auth"]
		if hasAuth && !hasAuthType {
			auth := resolve(authProp)
			st.AuthFields = sortedKeys(auth.Properties)
			st.RequiredAuthFields = sortedCopy(auth.Required)
			required := configRequired[schemaName] || contains(schema.Required, "auth")
			st.AuthRequired = required && len(st.AuthFields) > 0
		}
		out[normalizeTypeName(name)] = st
	}
	return out, nil
}

// SourceTypeFor returns the rules for sourceType, in any case or spelling the API accepts.
func SourceTypeFor(sourceTypes map[string]SourceType, sourceType string) (SourceType, bool) {
	st, ok := sourceTypes[normalizeTypeName(sourceType)]
	return st, ok
}

// refName returns the schema name a "#/components/schemas/<name>" reference points to.
func refName(ref string) string {
	return ref[strings.LastIndex(ref, "/")+1:]
}

func sortedKeys(m map[string]specSchema) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedCopy(s []string) []string {
	out := append([]string{}, s...)
	sort.Strings(out)
	return out
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

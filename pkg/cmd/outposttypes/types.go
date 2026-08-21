// Package outposttypes fetches and caches Outpost destination type schemas.
//
// Destination config and credential fields differ per type, and the set of
// types grows over time, so the CLI reads the schemas from the API rather than
// hardcoding them. Results are cached on disk for a short period to keep
// per-command latency down.
package outposttypes

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

var (
	cacheFilePrefix = "hookdeck_outpost_destination_types_"
	cacheTTL        = 24 * time.Hour
)

// Schema is one destination type's schema.
type Schema = hookdeck.OutpostDestinationTypeSchema

// Field is one configurable field within a schema.
type Field = hookdeck.OutpostDestinationTypeField

// FetchDestinationTypes returns the destination type schemas available to the
// active project, preferring a fresh on-disk cache.
//
// Callers should treat an error as non-fatal: warn and continue, letting the
// API validate the request instead. A stale local schema must never be the
// reason a valid command is rejected.
func FetchDestinationTypes(ctx context.Context, client *hookdeck.Client) ([]Schema, error) {
	cachePath := cachePathFor(client)

	if schemas, ok := readCache(cachePath); ok {
		return schemas, nil
	}

	schemas, err := client.ListOutpostDestinationTypes(ctx)
	if err != nil {
		return nil, err
	}

	writeCache(cachePath, schemas)

	return schemas, nil
}

// Find returns the schema for a destination type, matching case-insensitively.
func Find(schemas []Schema, destinationType string) (Schema, bool) {
	for _, schema := range schemas {
		if strings.EqualFold(schema.Type, destinationType) {
			return schema, true
		}
	}
	return Schema{}, false
}

// TypeNames returns the available type names, sorted, for help text and error
// messages.
func TypeNames(schemas []Schema) []string {
	names := make([]string, 0, len(schemas))
	for _, schema := range schemas {
		names = append(names, schema.Type)
	}
	sort.Strings(names)
	return names
}

// ValidateFields checks supplied values against a schema's field definitions,
// enforcing the schema's required fields.
//
// kind names the group being checked ("config" or "credential") so errors can
// point at the right flags. Only rules the schema states are enforced: missing
// required fields, unknown fields, values outside a declared option set, and
// values failing a declared pattern. Anything else is left to the API.
//
// This is create-time validation: the request specifies the whole object, so a
// required field nobody supplied is a request the API will reject. Update is a
// merge patch and must use ValidateSuppliedFields.
func ValidateFields(fields []Field, values map[string]interface{}, kind string) error {
	return validateFields(fields, values, kind, true)
}

// ApplyDefaultsForType looks up destinationType's schema and fills in its
// declared defaults for config fields the caller did not supply, returning the
// names of the fields it set.
//
// This lives here rather than in pkg/cmd because both entry points that create
// a destination need it. It was originally wired into the CLI create command
// only, which left the MCP server still creating rabbitmq destinations with
// tls unset — the same defect on a surface where nobody was watching stderr.
//
// A schema that cannot be fetched or found is not an error: the caller's input
// is passed through untouched and the API decides, which is the behaviour that
// existed before defaults were applied at all.
func ApplyDefaultsForType(ctx context.Context, client *hookdeck.Client, destinationType string, config map[string]interface{}) (map[string]interface{}, []string) {
	schemas, err := FetchDestinationTypes(ctx, client)
	if err != nil {
		return config, nil
	}
	schema, found := Find(schemas, destinationType)
	if !found {
		return config, nil
	}
	return ApplyDefaults(schema.ConfigFields, config)
}

// ApplyDefaults fills in the schema's declared default for any field the caller
// did not supply, and reports which keys it set.
//
// The schema publishes a default per field and the CLI prints it in help
// ("default: on"), but nothing was sending it, and the API treats an absent key
// as unset rather than applying the default itself. So a caller who read the
// help and omitted the flag got the opposite of what was advertised.
//
// That was worst for TLS. Following the CLI's own printed example for a
// rabbitmq destination stored tls: "false", and the connection carried its SASL
// credentials in the clear. Any field with a default has the same shape of
// problem; TLS is the one where it costs something.
//
// Create only. Update is a merge patch where an omitted key means "leave this
// alone", so filling in defaults there would silently rewrite fields the caller
// never mentioned.
//
// Values are sent verbatim as the schema declares them. The schema is not
// self-consistent about booleans — rabbitmq's tls default is "on" and kafka's
// is "true" — and the API normalises both to "true", so passing them through
// avoids inventing a mapping that could drift from whatever it accepts next.
func ApplyDefaults(fields []Field, values map[string]interface{}) (map[string]interface{}, []string) {
	var applied []string
	for _, field := range fields {
		if field.Default == "" {
			continue
		}
		if _, supplied := values[field.Key]; supplied {
			continue
		}
		if values == nil {
			values = map[string]interface{}{}
		}
		values[field.Key] = field.Default
		applied = append(applied, field.Key)
	}
	return values, applied
}

// ValidateSuppliedFields checks only the keys actually supplied.
//
// The update endpoint is a PATCH that leaves anything omitted alone, so
// enforcing required fields there rejects requests the API accepts. It made
// credential rotation impossible: `destination update des_x --credential
// secret=new` failed client-side with "--config url=<value> is required"
// before a request was ever sent, and the MCP path — which does no such
// validation — worked. A supplied field that is empty is still reported,
// because clearing a required field is not a partial update.
//
// Unknown keys, option sets and patterns are checked either way: those are
// wrong however the request is shaped, and the message is more useful than the
// API's.
func ValidateSuppliedFields(fields []Field, values map[string]interface{}, kind string) error {
	return validateFields(fields, values, kind, false)
}

func validateFields(fields []Field, values map[string]interface{}, kind string, requireAll bool) error {
	known := make(map[string]Field, len(fields))
	for _, field := range fields {
		known[field.Key] = field
	}

	var problems []string

	for _, field := range fields {
		if !field.Required {
			continue
		}
		value, present := values[field.Key]
		if !present && !requireAll {
			continue
		}
		if !present || isEmptyValue(value) {
			problems = append(problems, fmt.Sprintf("--%s %s=<value> is required", kind, field.Key))
		}
	}

	for key, value := range values {
		// A nested value came from a dotted path. The schema describes flat
		// fields today, so it cannot say whether a nested shape is valid, and
		// rejecting one here would block a command the API would have accepted.
		// Defer to the API, which is the authority.
		if _, nested := value.(map[string]interface{}); nested {
			continue
		}

		field, ok := known[key]
		if !ok {
			problems = append(problems, fmt.Sprintf("%q is not a valid %s field", key, kind))
			continue
		}

		text, isText := value.(string)
		if !isText || text == "" {
			continue
		}

		if options := field.OptionValues(); len(options) > 0 && !containsFold(options, text) {
			problems = append(problems, fmt.Sprintf("--%s %s must be one of: %s",
				kind, key, strings.Join(options, ", ")))
			continue
		}

		if field.Pattern != "" {
			// A schema pattern the CLI cannot compile is a problem with the
			// schema, not the user's input, so it is ignored rather than
			// reported as a validation failure.
			if re, err := regexp.Compile(field.Pattern); err == nil && !re.MatchString(text) {
				problems = append(problems, fmt.Sprintf("--%s %s does not match the expected format (%s)",
					kind, key, field.Pattern))
			}
		}
	}

	if len(problems) == 0 {
		return nil
	}

	sort.Strings(problems)
	return fmt.Errorf("%s", strings.Join(problems, "\n"))
}

func containsFold(options []string, value string) bool {
	for _, option := range options {
		if strings.EqualFold(option, value) {
			return true
		}
	}
	return false
}

func isEmptyValue(value interface{}) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(v) == ""
	default:
		return false
	}
}

// cachePathFor derives a cache file per API host and project. Destination types
// come from the project's own deployment, so a single shared cache file would
// serve one project's schemas to another.
func cachePathFor(client *hookdeck.Client) string {
	var key string
	if client != nil {
		if client.BaseURL != nil {
			key = client.BaseURL.Host
		}
		key += "|" + client.ProjectID
	}

	hash := fnv.New64a()
	_, _ = hash.Write([]byte(key))

	return filepath.Join(os.TempDir(), fmt.Sprintf("%s%x.json", cacheFilePrefix, hash.Sum64()))
}

func readCache(path string) ([]Schema, bool) {
	info, err := os.Stat(path)
	if err != nil || time.Since(info.ModTime()) >= cacheTTL {
		return nil, false
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}

	var schemas []Schema
	if err := json.Unmarshal(data, &schemas); err != nil || len(schemas) == 0 {
		return nil, false
	}

	return schemas, true
}

// writeCache stores schemas for later runs. Failures are ignored: the cache is
// an optimisation, and a read-only or full temp dir must not break the command.
func writeCache(path string, schemas []Schema) {
	data, err := json.Marshal(schemas)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}

// LookupCached returns a destination type's schema from the on-disk cache only,
// never touching the network.
//
// It exists for paths that must not block or fail, such as augmenting --help:
// a miss simply means the caller shows less, not that anything went wrong.
func LookupCached(client *hookdeck.Client, destinationType string) (Schema, bool) {
	schemas, ok := readCache(cachePathFor(client))
	if !ok {
		return Schema{}, false
	}
	return Find(schemas, destinationType)
}

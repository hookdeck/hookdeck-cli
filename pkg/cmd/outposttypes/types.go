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

// ValidateFields checks supplied values against a schema's field definitions.
//
// kind names the group being checked ("config" or "credential") so errors can
// point at the right flags. Only rules the schema states are enforced: missing
// required fields, unknown fields, values outside a declared option set, and
// values failing a declared pattern. Anything else is left to the API.
func ValidateFields(fields []Field, values map[string]interface{}, kind string) error {
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
		if !present || isEmptyValue(value) {
			problems = append(problems, fmt.Sprintf("--%s-%s is required", kind, flagName(field.Key)))
		}
	}

	for key, value := range values {
		field, ok := known[key]
		if !ok {
			problems = append(problems, fmt.Sprintf("--%s-%s is not a valid %s field", kind, flagName(key), kind))
			continue
		}

		text, isText := value.(string)
		if !isText || text == "" {
			continue
		}

		if options := field.OptionValues(); len(options) > 0 && !containsFold(options, text) {
			problems = append(problems, fmt.Sprintf("--%s-%s must be one of: %s",
				kind, flagName(key), strings.Join(options, ", ")))
			continue
		}

		if field.Pattern != "" {
			// A schema pattern the CLI cannot compile is a problem with the
			// schema, not the user's input, so it is ignored rather than
			// reported as a validation failure.
			if re, err := regexp.Compile(field.Pattern); err == nil && !re.MatchString(text) {
				problems = append(problems, fmt.Sprintf("--%s-%s does not match the expected format (%s)",
					kind, flagName(key), field.Pattern))
			}
		}
	}

	if len(problems) == 0 {
		return nil
	}

	sort.Strings(problems)
	return fmt.Errorf("%s", strings.Join(problems, "\n"))
}

// flagName converts a schema field key to the CLI flag spelling.
func flagName(key string) string {
	return strings.ReplaceAll(key, "_", "-")
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

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"

	"github.com/hookdeck/hookdeck-cli/pkg/cmd/outposttypes"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// outpostDestinationFieldFlags carries the type-specific parts of a destination.
//
// Destination config and credential fields differ per type and change server
// side, so they cannot be declared as individual Cobra flags: the CLI does not
// know the field set until it has fetched the schema, and doing that at flag
// registration would mean a network call before every command. They are taken
// as repeatable key=value pairs instead and validated against the fetched
// schema, so a wrong key is still rejected with the right message.
//
// Use 'hookdeck outpost destination-type get <type>' to see the fields a type
// accepts.
type outpostDestinationFieldFlags struct {
	config          []string
	credential      []string
	configFile      string
	credentialsFile string
	topics          string
	filter          string
	filterFile      string
	metadata        []string
	metadataFile    string
}

func addOutpostDestinationFieldFlags(cmd *cobra.Command, f *outpostDestinationFieldFlags) {
	cmd.Flags().StringArrayVar(&f.config, "config", nil, "Config field as key=value (repeatable), e.g. --config url=https://example.com")
	cmd.Flags().StringArrayVar(&f.credential, "credential", nil, "Credential field as key=value (repeatable)")
	cmd.Flags().StringVar(&f.configFile, "config-file", "", "Path to a JSON file of config fields")
	cmd.Flags().StringVar(&f.credentialsFile, "credentials-file", "", "Path to a JSON file of credential fields")
	cmd.Flags().StringVar(&f.topics, "topics", "", `Topics to subscribe to, comma-separated, or "*" for all`)
	cmd.Flags().StringVar(&f.filter, "filter", "", "Event filter as a JSON object")
	cmd.Flags().StringVar(&f.filterFile, "filter-file", "", "Path to a JSON file containing an event filter")
	cmd.Flags().StringArrayVar(&f.metadata, "metadata", nil, "Metadata as key=value (repeatable)")
	cmd.Flags().StringVar(&f.metadataFile, "metadata-file", "", "Path to a JSON file of metadata key/value pairs")
}

func (f *outpostDestinationFieldFlags) validate() error {
	if len(f.config) > 0 && f.configFile != "" {
		return fmt.Errorf("--config and --config-file cannot be used together")
	}
	if len(f.credential) > 0 && f.credentialsFile != "" {
		return fmt.Errorf("--credential and --credentials-file cannot be used together")
	}
	if f.filter != "" && f.filterFile != "" {
		return fmt.Errorf("--filter and --filter-file cannot be used together")
	}
	if len(f.metadata) > 0 && f.metadataFile != "" {
		return fmt.Errorf("--metadata and --metadata-file cannot be used together")
	}
	return nil
}

func (f *outpostDestinationFieldFlags) hasAny() bool {
	return len(f.config) > 0 || len(f.credential) > 0 || f.configFile != "" ||
		f.credentialsFile != "" || f.topics != "" || f.filter != "" || f.filterFile != "" ||
		len(f.metadata) > 0 || f.metadataFile != ""
}

func (f *outpostDestinationFieldFlags) resolveConfig() (map[string]interface{}, error) {
	return resolveOutpostFieldMap(f.config, f.configFile, "config")
}

func (f *outpostDestinationFieldFlags) resolveCredentials() (map[string]interface{}, error) {
	return resolveOutpostFieldMap(f.credential, f.credentialsFile, "credential")
}

// resolveTopics returns the topics to send. Nil means "leave unchanged", which
// on update is the difference between not touching topics and clearing them.
func (f *outpostDestinationFieldFlags) resolveTopics() hookdeck.OutpostTopics {
	if f.topics == "" {
		return nil
	}
	if strings.TrimSpace(f.topics) == hookdeck.OutpostTopicsWildcard {
		return hookdeck.OutpostTopics{hookdeck.OutpostTopicsWildcard}
	}
	return hookdeck.OutpostTopics(splitCommaList(f.topics))
}

func (f *outpostDestinationFieldFlags) resolveMetadata() (map[string]string, error) {
	return resolveOutpostMetadata(f.metadata, f.metadataFile)
}

func (f *outpostDestinationFieldFlags) resolveFilter() (map[string]interface{}, error) {
	raw := f.filter
	if f.filterFile != "" {
		contents, err := os.ReadFile(f.filterFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read --filter-file: %w", err)
		}
		raw = string(contents)
	}
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	var filter map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &filter); err != nil {
		return nil, fmt.Errorf("filter must be a JSON object: %w", err)
	}
	return filter, nil
}

// resolveOutpostMetadata reads --metadata pairs or a --metadata-file into the
// string map the API expects.
//
// Metadata is a plain string map on every Outpost resource that has it, so
// unlike config and credentials there is no dotted-path nesting here: a dot in
// a key is part of the key.
//
// Nil means "not supplied", which on update is the difference between leaving
// metadata alone and replacing it.
func resolveOutpostMetadata(pairs []string, file string) (map[string]string, error) {
	if file != "" {
		contents, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read --metadata-file: %w", err)
		}
		var metadata map[string]string
		if err := json.Unmarshal(contents, &metadata); err != nil {
			return nil, fmt.Errorf("--metadata-file must contain a JSON object of string values: %w", err)
		}
		return metadata, nil
	}

	if len(pairs) == 0 {
		return nil, nil
	}

	metadata := make(map[string]string, len(pairs))
	for _, entry := range pairs {
		key, value, found := strings.Cut(entry, "=")
		key = strings.TrimSpace(key)
		if !found || key == "" {
			return nil, fmt.Errorf("--metadata %q must be in key=value form", entry)
		}
		metadata[key] = value
	}
	return metadata, nil
}

// applyOutpostDestinationDefaults fills in the schema's declared defaults for
// config fields the caller did not supply, and says so on stderr.
//
// The CLI prints these defaults in help ("default: on") but was not sending
// them, and the API treats an absent key as unset rather than applying the
// default itself — so following our own documented example for a rabbitmq
// destination stored tls: "false" and sent its SASL credentials in the clear.
//
// Create only. Update is a merge patch where an omitted key means "leave this
// alone", so filling defaults there would rewrite fields nobody mentioned.
//
// Failing to fetch the schema is not fatal here, matching the validation path:
// the CLI warns and lets the API decide. It does mean the default is not
// applied, which is the pre-existing behaviour rather than a new risk.
func applyOutpostDestinationDefaults(ctx context.Context, destinationType string, config map[string]interface{}) map[string]interface{} {
	config, applied := outposttypes.ApplyDefaultsForType(ctx, Config.GetOutpostAPIClient(), destinationType, config)
	if len(applied) > 0 {
		sort.Strings(applied)
		parts := make([]string, 0, len(applied))
		for _, key := range applied {
			parts = append(parts, fmt.Sprintf("%s=%v", key, config[key]))
		}
		// Say it out loud. A silently applied default is how the opposite
		// problem starts: someone who wants TLS off needs to know they have to
		// ask for it.
		fmt.Fprintf(os.Stderr, "Using defaults from the %s schema for fields you did not set: %s\n",
			destinationType, strings.Join(parts, ", "))
	}
	return config
}

// resolveOutpostFieldMap merges key=value pairs or a JSON file into one map.
func resolveOutpostFieldMap(pairs []string, file, kind string) (map[string]interface{}, error) {
	if file != "" {
		contents, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read --%s file: %w", kind, err)
		}
		var values map[string]interface{}
		if err := json.Unmarshal(contents, &values); err != nil {
			return nil, fmt.Errorf("the %s file must contain a JSON object: %w", kind, err)
		}
		return values, nil
	}

	if len(pairs) == 0 {
		return nil, nil
	}

	values := make(map[string]interface{}, len(pairs))
	for _, pair := range pairs {
		key, value, found := strings.Cut(pair, "=")
		key = strings.TrimSpace(key)
		if !found || key == "" {
			return nil, fmt.Errorf("--%s %q must be in key=value form", kind, pair)
		}
		if err := setNestedValue(values, key, value, kind); err != nil {
			return nil, err
		}
	}
	return values, nil
}

// setNestedValue assigns value at a dotted path, creating intermediate maps.
//
// Outpost's destination config is flat today — every field is a top-level string
// — so in practice this is a plain assignment. It supports paths because
// destination types are defined by the deployment rather than the CLI: if a
// nested type ships, `--config a.b=c` expresses it with no CLI release, which is
// the whole point of not hardcoding a server-owned schema.
//
// A literal dot in a key can be escaped as `\.`. No current field key in either
// product contains one, so this exists to avoid painting us into a corner rather
// than to solve a present problem.
func setNestedValue(target map[string]interface{}, key, value, kind string) error {
	segments := splitDottedPath(key)

	for i, segment := range segments {
		if segment == "" {
			return fmt.Errorf("--%s %q has an empty path segment", kind, key)
		}

		if i == len(segments)-1 {
			target[segment] = value
			break
		}

		switch existing := target[segment].(type) {
		case nil:
			next := map[string]interface{}{}
			target[segment] = next
			target = next
		case map[string]interface{}:
			target = existing
		default:
			// e.g. --config a=1 --config a.b=2, where "a" cannot be both.
			return fmt.Errorf("--%s %q conflicts with an earlier value for %q", kind, key, segment)
		}
	}

	return nil
}

// splitDottedPath splits on unescaped dots, so `a\.b` stays a single segment.
func splitDottedPath(key string) []string {
	var segments []string
	var current strings.Builder

	for i := 0; i < len(key); i++ {
		switch {
		case key[i] == '\\' && i+1 < len(key) && key[i+1] == '.':
			current.WriteByte('.')
			i++
		case key[i] == '.':
			segments = append(segments, current.String())
			current.Reset()
		default:
			current.WriteByte(key[i])
		}
	}

	return append(segments, current.String())
}

// fieldValidationMode selects between create-time and update-time rules.
type fieldValidationMode int

const (
	// validateForCreate enforces the schema's required fields. Create specifies
	// the whole destination, so one that is missing will be rejected by the API
	// anyway and the local message names the flag.
	validateForCreate fieldValidationMode = iota
	// validateForUpdate checks only the keys supplied. Update is a merge patch,
	// so applying create-time rules to it rejects requests the API accepts.
	validateForUpdate
)

// validateOutpostDestinationFields checks config and credentials against the
// destination type's schema.
//
// Per AGENTS.md, a schema that cannot be fetched must not block the command: the
// API is the authority, so this warns and lets the request through.
func validateOutpostDestinationFields(ctx context.Context, destinationType string, config, credentials map[string]interface{}, mode fieldValidationMode) error {
	client := Config.GetOutpostAPIClient()

	schemas, err := outposttypes.FetchDestinationTypes(ctx, client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not fetch destination type schemas (%v); continuing without local validation.\n", err)
		return nil
	}

	schema, found := outposttypes.Find(schemas, destinationType)
	if !found {
		return fmt.Errorf("unknown destination type %q. Available types: %s",
			destinationType, strings.Join(outposttypes.TypeNames(schemas), ", "))
	}

	validate := outposttypes.ValidateFields
	if mode == validateForUpdate {
		validate = outposttypes.ValidateSuppliedFields
	}

	if err := validate(schema.ConfigFields, config, "config"); err != nil {
		return fmt.Errorf("%w\n\nRun 'hookdeck outpost destination-type get %s' to see the fields this type accepts", err, destinationType)
	}
	if err := validate(schema.CredentialFields, credentials, "credential"); err != nil {
		return fmt.Errorf("%w\n\nRun 'hookdeck outpost destination-type get %s' to see the fields this type accepts", err, destinationType)
	}

	return nil
}

// printOutpostDestination renders a destination for text output.
func printOutpostDestination(destination *hookdeck.OutpostDestination, indent string) {
	color := ansi.Color(os.Stdout)

	fmt.Printf("%s%s\n", indent, color.Green(destination.ID))
	fmt.Printf("%s  Type: %s\n", indent, destination.Type)

	if destination.Topics.IsWildcard() {
		fmt.Printf("%s  Topics: all\n", indent)
	} else if len(destination.Topics) > 0 {
		fmt.Printf("%s  Topics: %s\n", indent, strings.Join(destination.Topics, ", "))
	}

	for _, key := range sortedKeys(destination.Config) {
		fmt.Printf("%s  %s: %v\n", indent, key, destination.Config[key])
	}

	if destination.Disabled() {
		fmt.Printf("%s  Status: %s\n", indent, color.Red("disabled"))
	} else {
		fmt.Printf("%s  Status: %s\n", indent, color.Green("active"))
	}
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

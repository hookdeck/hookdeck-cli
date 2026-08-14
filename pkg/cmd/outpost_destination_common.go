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
}

func addOutpostDestinationFieldFlags(cmd *cobra.Command, f *outpostDestinationFieldFlags) {
	cmd.Flags().StringArrayVar(&f.config, "config", nil, "Config field as key=value (repeatable), e.g. --config url=https://example.com")
	cmd.Flags().StringArrayVar(&f.credential, "credential", nil, "Credential field as key=value (repeatable)")
	cmd.Flags().StringVar(&f.configFile, "config-file", "", "Path to a JSON file of config fields")
	cmd.Flags().StringVar(&f.credentialsFile, "credentials-file", "", "Path to a JSON file of credential fields")
	cmd.Flags().StringVar(&f.topics, "topics", "", `Topics to subscribe to, comma-separated, or "*" for all`)
	cmd.Flags().StringVar(&f.filter, "filter", "", "Event filter as a JSON object")
	cmd.Flags().StringVar(&f.filterFile, "filter-file", "", "Path to a JSON file containing an event filter")
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
	return nil
}

func (f *outpostDestinationFieldFlags) hasAny() bool {
	return len(f.config) > 0 || len(f.credential) > 0 || f.configFile != "" ||
		f.credentialsFile != "" || f.topics != "" || f.filter != "" || f.filterFile != ""
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
		values[key] = value
	}
	return values, nil
}

// validateOutpostDestinationFields checks config and credentials against the
// destination type's schema.
//
// Per AGENTS.md, a schema that cannot be fetched must not block the command: the
// API is the authority, so this warns and lets the request through.
func validateOutpostDestinationFields(ctx context.Context, destinationType string, config, credentials map[string]interface{}) error {
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

	if err := outposttypes.ValidateFields(schema.ConfigFields, config, "config"); err != nil {
		return fmt.Errorf("%w\n\nRun 'hookdeck outpost destination-type get %s' to see the fields this type accepts", err, destinationType)
	}
	if err := outposttypes.ValidateFields(schema.CredentialFields, credentials, "credential"); err != nil {
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

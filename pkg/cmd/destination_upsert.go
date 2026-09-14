package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type destinationUpsertCmd struct {
	cmd *cobra.Command

	name        string
	description string
	destType    string
	url         string
	cliPath     string
	config      string
	configFile  string
	dryRun      bool
	output      string

	destinationConfigFlags
}

func newDestinationUpsertCmd() *destinationUpsertCmd {
	dc := &destinationUpsertCmd{}

	dc.cmd = &cobra.Command{
		Use:   "upsert <name>",
		Args:  validators.ExactArgs(1),
		Short: ShortUpsert(ResourceDestination),
		Long: LongUpsertIntro(ResourceDestination) + `

Examples:
  hookdeck gateway destination upsert my-api --type HTTP --url https://api.example.com/webhooks
  hookdeck gateway destination upsert local-cli --type CLI --cli-path /webhooks
  hookdeck gateway destination upsert my-api --description "Updated" --dry-run`,
		PreRunE: dc.validateFlags,
		RunE:    dc.runDestinationUpsertCmd,
	}

	dc.cmd.Flags().StringVar(&dc.description, "description", "", "Destination description")
	dc.cmd.Flags().StringVar(&dc.destType, "type", "", "Destination type (HTTP, CLI, MOCK_API)")
	dc.cmd.Flags().StringVar(&dc.url, "url", "", "URL for HTTP destinations")
	dc.cmd.Flags().StringVar(&dc.cliPath, "cli-path", "", "Path for CLI destinations")
	dc.cmd.Flags().StringVar(&dc.config, "config", "", "JSON object for the whole destination config; cannot be combined with the individual config flags")
	dc.cmd.Flags().StringVar(&dc.configFile, "config-file", "", "Path to a JSON file holding the whole destination config; cannot be combined with the individual config flags")
	dc.cmd.Flags().StringVar(&dc.AuthMethod, "auth-method", "", "Auth method (hookdeck, bearer, basic, api_key, custom_signature)")
	dc.cmd.Flags().StringVar(&dc.BearerToken, "bearer-token", "", "Bearer token for destination auth")
	dc.cmd.Flags().StringVar(&dc.BasicAuthUser, "basic-auth-user", "", "Username for Basic auth")
	dc.cmd.Flags().StringVar(&dc.BasicAuthPass, "basic-auth-pass", "", "Password for Basic auth")
	dc.cmd.Flags().StringVar(&dc.APIKey, "api-key", "", "API key for destination auth")
	dc.cmd.Flags().StringVar(&dc.APIKeyHeader, "api-key-header", "", "Header/key name for API key")
	dc.cmd.Flags().StringVar(&dc.APIKeyTo, "api-key-to", "header", "Where to send API key (header or query)")
	dc.cmd.Flags().StringVar(&dc.CustomSignatureSecret, "custom-signature-secret", "", "Signing secret for custom signature")
	dc.cmd.Flags().StringVar(&dc.CustomSignatureKey, "custom-signature-key", "", "Key/header name for custom signature")
	addDestinationDeliveryPolicyFlags(dc.cmd, &dc.destinationConfigFlags)
	dc.cmd.Flags().StringVar(&dc.HTTPMethod, "http-method", "", "HTTP method for HTTP destinations")
	dc.cmd.Flags().BoolVar(&dc.dryRun, "dry-run", false, "Preview changes without applying")
	dc.cmd.Flags().StringVar(&dc.output, "output", "", "Output format (json)")

	return dc
}

func (dc *destinationUpsertCmd) validateFlags(cmd *cobra.Command, args []string) error {
	// An explicitly empty value for a secret or identity flag cannot mean
	// anything, and is almost always an unexported shell variable (#335).
	if err := rejectEmptyFlags(cmd); err != nil {
		return err
	}

	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	dc.name = args[0]
	if dc.config != "" && dc.configFile != "" {
		return fmt.Errorf("cannot use both --config and --config-file")
	}
	// --config / --config-file supply the whole config, so an individual config
	// flag alongside one of them is a conflict, not an override. Refused rather
	// than silently resolved: the three commands resolved it three different
	// ways and one of them dropped --url without a word.
	if err := rejectConfigJSONWithIndividualFlags(cmd, dc.config, dc.configFile); err != nil {
		return err
	}
	// Nothing below applies on the config-JSON path: the individual flags are
	// refused above, so there is nothing left for them to validate.
	if dc.config != "" || dc.configFile != "" {
		return nil
	}
	return dc.destinationConfigFlags.validateDeliveryPolicyFlags("")
}

func (dc *destinationUpsertCmd) runDestinationUpsertCmd(cmd *cobra.Command, args []string) error {
	client := Config.GetAPIClient()
	ctx := context.Background()

	req, err := dc.buildUpsertRequest(ctx, client)
	if err != nil {
		return err
	}

	if dc.dryRun {
		params := map[string]string{"name": dc.name}
		existing, err := client.ListDestinations(ctx, params)
		if err != nil {
			return fmt.Errorf("dry-run: failed to check existing destination: %w", err)
		}
		if existing.Models != nil && len(existing.Models) > 0 {
			fmt.Printf("-- Dry Run: UPDATE --\nDestination '%s' (%s) would be updated.\n", dc.name, existing.Models[0].ID)
		} else {
			fmt.Printf("-- Dry Run: CREATE --\nDestination '%s' would be created.\n", dc.name)
		}
		return nil
	}

	dst, err := client.UpsertDestination(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to upsert destination: %w", err)
	}

	if dc.output == "json" {
		jsonBytes, err := json.MarshalIndent(dst, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal destination to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	fmt.Printf(SuccessCheck + " Destination upserted successfully\n\n")
	fmt.Printf("Destination: %s (%s)\n", dst.Name, dst.ID)
	fmt.Printf("Type: %s\n", dst.Type)
	if u := dst.GetHTTPURL(); u != nil {
		fmt.Printf("URL: %s\n", *u)
	}
	return nil
}

// buildUpsertRequest assembles the PUT body, consulting the stored destination
// where a flag alone cannot answer the question. Separated from the command so
// the guards it applies are reachable from a test with a real HTTP client.
func (dc *destinationUpsertCmd) buildUpsertRequest(ctx context.Context, client *hookdeck.Client) (*hookdeck.DestinationCreateRequest, error) {
	dc.destinationConfigFlags.URL = dc.url
	dc.destinationConfigFlags.CliPath = dc.cliPath

	// The stored destination answers two questions below: what type it is, which
	// decides how every type-dependent flag is read when --type is omitted — the
	// delivery-policy guard and the --url/--cli-path config fields alike — and
	// what delivery-group overrides it holds, so a bare groups object does not
	// destroy them. Fetch it at most once, and only when it is needed.
	var (
		existingDest    *hookdeck.Destination
		fetchedExisting bool
	)
	lookupExisting := func() (*hookdeck.Destination, error) {
		if fetchedExisting {
			return existingDest, nil
		}
		found, err := fetchDestinationByName(ctx, client, dc.name)
		if err != nil {
			return nil, err
		}
		existingDest, fetchedExisting = found, true
		return found, nil
	}

	// --type is normally omitted on upsert, so the stored type has to be
	// resolved before the config is built or nothing that depends on it works:
	// the delivery-policy guard never fires, and --url and --cli-path are left
	// out of the request body entirely (#406).
	resolvedType, err := resolveDestinationType(
		dc.destType,
		dc.config != "" || dc.configFile != "",
		&dc.destinationConfigFlags,
		lookupExisting,
	)
	if err != nil {
		return nil, err
	}

	config, err := buildDestinationConfigFromFlags(dc.config, dc.configFile, resolvedType, &dc.destinationConfigFlags)
	if err != nil {
		return nil, err
	}
	if err := rejectDeliveryPolicyInConfigForCLI(resolvedType, config, ""); err != nil {
		return nil, err
	}

	// No overlay here. It existed to put --url and --cli-path back on top of a
	// --config body, and only fired when --type was passed, because
	// resolveDestinationType returns early on the --config path: that is how
	// `upsert --config '{"url":...}' --url ...` exited 0 having sent the old
	// URL while the same flags with --type HTTP sent the new one, and `update`
	// disagreed with both. The combination is now refused in validateFlags, so
	// every config field arrives through the builder above, once.
	rt := strings.ToUpper(resolvedType)

	req := &hookdeck.DestinationCreateRequest{
		Name: dc.name,
	}
	if dc.description != "" {
		req.Description = &dc.description
	}
	// Only send a type the user actually asked for. The resolved type is used to
	// decide which config fields are valid, but asserting it back on the request
	// would make this a read-modify-write: if the destination's type changed
	// between the lookup and this PUT, we would silently revert it. The API keeps
	// the stored type when the field is absent, verified against the live API.
	if dc.destType != "" {
		req.Type = rt
	}
	if len(config) > 0 {
		req.Config = config
	}

	// API requires config on PUT. When doing partial update (e.g. only --description), fetch existing and merge.
	// A groups object sent without overrides also needs the stored config, because
	// the API replaces groups wholesale and would drop the overrides with it.
	if err := applyStoredDestinationConfig(dc.name, req, lookupExisting); err != nil {
		return nil, err
	}

	return req, nil
}

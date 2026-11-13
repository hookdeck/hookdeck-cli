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

type sourceUpsertCmd struct {
	cmd *cobra.Command

	name        string
	description string
	sourceType  string
	config      string
	configFile  string
	dryRun      bool
	output      string

	sourceConfigFlags
}

func newSourceUpsertCmd() *sourceUpsertCmd {
	sc := &sourceUpsertCmd{}

	sc.cmd = &cobra.Command{
		Use:   "upsert <name>",
		Args:  validators.ExactArgs(1),
		Short: ShortUpsert(ResourceSource),
		Long: LongUpsertIntro(ResourceSource) + `

When --type is omitted and the source already exists, its current type is kept,
along with its verification unless you pass new auth flags or config. When the
source does not exist, it is created as a WEBHOOK source. Pass --type to change
an existing source's type.

` + LongSourceAuthType("--config") + `

Examples:
  hookdeck gateway source upsert my-webhook --type WEBHOOK
  hookdeck gateway source upsert stripe-prod --type STRIPE --config '{"webhook_secret":"whsec_xxx"}'
  hookdeck gateway source upsert my-webhook --description "Updated" --dry-run`,
		PreRunE: sc.validateFlags,
		RunE:    sc.runSourceUpsertCmd,
	}

	sc.cmd.Flags().StringVar(&sc.description, "description", "", "Source description")
	sc.cmd.Flags().StringVar(&sc.sourceType, "type", "", "Source type (e.g. WEBHOOK, STRIPE)")
	sc.cmd.Flags().StringVar(&sc.config, "config", "", "JSON object for source config (overrides individual flags if set)")
	sc.cmd.Flags().StringVar(&sc.configFile, "config-file", "", "Path to JSON file for source config (overrides individual flags if set)")
	sc.cmd.Flags().StringVar(&sc.WebhookSecret, "webhook-secret", "", "Webhook secret the source verifies signatures with; sent as webhook_secret_key, plus HMAC settings on WEBHOOK and HTTP only")
	sc.cmd.Flags().StringVar(&sc.APIKey, "api-key", "", "API key for source authentication")
	sc.cmd.Flags().StringVar(&sc.BasicAuthUser, "basic-auth-user", "", "Username for Basic authentication")
	sc.cmd.Flags().StringVar(&sc.BasicAuthPass, "basic-auth-pass", "", "Password for Basic authentication")
	sc.cmd.Flags().StringVar(&sc.HMACSecret, "hmac-secret", "", "HMAC secret for signature verification")
	sc.cmd.Flags().StringVar(&sc.HMACAlgo, "hmac-algo", "", "HMAC algorithm (SHA256, etc.)")
	sc.cmd.Flags().StringVar(&sc.AllowedHTTPMethods, "allowed-http-methods", "", "Comma-separated allowed HTTP methods (GET, POST, PUT, PATCH, DELETE)")
	sc.cmd.Flags().StringVar(&sc.CustomResponseBody, "custom-response-body", "", "Custom response body (max 1000 chars)")
	sc.cmd.Flags().StringVar(&sc.CustomResponseType, "custom-response-content-type", "", "Custom response content type (json, text, xml)")
	sc.cmd.Flags().BoolVar(&sc.dryRun, "dry-run", false, "Preview changes without applying")
	sc.cmd.Flags().Var(newOutputFormatValue(&sc.output), "output", "Output format (json)")

	return sc
}

func (sc *sourceUpsertCmd) validateFlags(cmd *cobra.Command, args []string) error {
	// An explicitly empty value for a secret or identity flag cannot mean
	// anything, and is almost always an unexported shell variable (#335).
	if err := rejectEmptyFlags(cmd); err != nil {
		return err
	}

	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	sc.name = args[0]
	if sc.config != "" && sc.configFile != "" {
		return fmt.Errorf("cannot use both --config and --config-file")
	}
	auth := sourceAuthFlags{
		WebhookSecret: sc.WebhookSecret,
		APIKey:        sc.APIKey,
		BasicAuthUser: sc.BasicAuthUser,
		BasicAuthPass: sc.BasicAuthPass,
		HMACSecret:    sc.HMACSecret,
	}
	return validateSourceAuthFromSpec(sc.sourceType, sc.config != "" || sc.configFile != "", auth, "")
}

func (sc *sourceUpsertCmd) runSourceUpsertCmd(cmd *cobra.Command, args []string) error {
	client := Config.GetAPIClient()
	ctx := context.Background()

	// Look the source up when --dry-run needs to preview it, or when --type is
	// omitted. PUT /sources validates against the create schema, where type
	// defaults to WEBHOOK, so an upsert body without a type converts an
	// existing source of any other type into a WEBHOOK source and drops its
	// verification. Sending the stored type keeps it.
	var existing *hookdeck.Source
	if sc.dryRun || sc.sourceType == "" {
		var err error
		existing, err = findSourceByName(ctx, client, sc.name)
		if err != nil {
			if sc.dryRun {
				return fmt.Errorf("dry-run: failed to check existing source: %w", err)
			}
			return fmt.Errorf("failed to look up existing source %q to keep its type (pass --type to skip the lookup): %w", sc.name, err)
		}
	}

	sourceType := strings.ToUpper(sc.sourceType)
	if sourceType == "" && existing != nil {
		sourceType = existing.Type
	}

	// Build config for the type the API will apply: the stored or given type,
	// or WEBHOOK when a new source is created without --type.
	config, err := buildSourceConfigFromFlags(sc.config, sc.configFile, &sc.sourceConfigFlags, sourceTypeOrAPIDefault(sourceType))
	if err != nil {
		return err
	}

	req := &hookdeck.SourceCreateRequest{
		Name: sc.name,
		// Empty only when --type is omitted and no source has this name; the
		// API then creates a WEBHOOK source.
		Type: sourceType,
	}
	if sc.description != "" {
		req.Description = &sc.description
	}
	if len(config) > 0 {
		req.Config = config
	}

	if sc.dryRun {
		if existing != nil {
			fmt.Printf("-- Dry Run: UPDATE --\nSource '%s' (%s) would be updated.\n", sc.name, existing.ID)
		} else {
			fmt.Printf("-- Dry Run: CREATE --\nSource '%s' would be created.\n", sc.name)
		}
		return nil
	}

	src, err := client.UpsertSource(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to upsert source: %w", err)
	}

	if sc.output == "json" {
		jsonBytes, err := json.MarshalIndent(src, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal source to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	fmt.Printf(SuccessCheck + " Source upserted successfully\n\n")
	fmt.Printf("Source: %s (%s)\n", src.Name, src.ID)
	fmt.Printf("Type:  %s\n", src.Type)
	fmt.Printf("URL:   %s\n", src.URL)
	return nil
}

// findSourceByName returns the source with exactly this name, or nil when there
// is none. disabled=true includes disabled sources: the API upserts a disabled
// source by name too, so missing one here would convert it to WEBHOOK.
func findSourceByName(ctx context.Context, client *hookdeck.Client, name string) (*hookdeck.Source, error) {
	resp, err := client.ListSources(ctx, map[string]string{"name": name, "disabled": "true"})
	if err != nil {
		return nil, err
	}
	for i := range resp.Models {
		if resp.Models[i].Name == name {
			return &resp.Models[i], nil
		}
	}
	return nil, nil
}

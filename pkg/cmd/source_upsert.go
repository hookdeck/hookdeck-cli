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
		Short: "Create or update a source by name",
		Long: `Create a new source or update an existing one by name (idempotent).

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
	sc.cmd.Flags().StringVar(&sc.WebhookSecret, "webhook-secret", "", "Webhook secret for source verification (e.g., Stripe)")
	sc.cmd.Flags().StringVar(&sc.APIKey, "api-key", "", "API key for source authentication")
	sc.cmd.Flags().StringVar(&sc.BasicAuthUser, "basic-auth-user", "", "Username for Basic authentication")
	sc.cmd.Flags().StringVar(&sc.BasicAuthPass, "basic-auth-pass", "", "Password for Basic authentication")
	sc.cmd.Flags().StringVar(&sc.HMACSecret, "hmac-secret", "", "HMAC secret for signature verification")
	sc.cmd.Flags().StringVar(&sc.HMACAlgo, "hmac-algo", "", "HMAC algorithm (SHA256, etc.)")
	sc.cmd.Flags().StringVar(&sc.AllowedHTTPMethods, "allowed-http-methods", "", "Comma-separated allowed HTTP methods (GET, POST, PUT, PATCH, DELETE)")
	sc.cmd.Flags().StringVar(&sc.CustomResponseBody, "custom-response-body", "", "Custom response body (max 1000 chars)")
	sc.cmd.Flags().StringVar(&sc.CustomResponseType, "custom-response-content-type", "", "Custom response content type (json, text, xml)")
	sc.cmd.Flags().BoolVar(&sc.dryRun, "dry-run", false, "Preview changes without applying")
	sc.cmd.Flags().StringVar(&sc.output, "output", "", "Output format (json)")

	return sc
}

func (sc *sourceUpsertCmd) validateFlags(cmd *cobra.Command, args []string) error {
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

	config, err := buildSourceConfigFromFlags(sc.config, sc.configFile, &sc.sourceConfigFlags)
	if err != nil {
		return err
	}

	req := &hookdeck.SourceCreateRequest{
		Name: sc.name,
	}
	if sc.description != "" {
		req.Description = &sc.description
	}
	if sc.sourceType != "" {
		req.Type = strings.ToUpper(sc.sourceType)
	}
	if len(config) > 0 {
		req.Config = config
	}

	if sc.dryRun {
		params := map[string]string{"name": sc.name}
		existing, err := client.ListSources(ctx, params)
		if err != nil {
			return fmt.Errorf("dry-run: failed to check existing source: %w", err)
		}
		if existing.Models != nil && len(existing.Models) > 0 {
			fmt.Printf("-- Dry Run: UPDATE --\nSource '%s' (%s) would be updated.\n", sc.name, existing.Models[0].ID)
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

	fmt.Printf("✔ Source upserted successfully\n\n")
	fmt.Printf("Source: %s (%s)\n", src.Name, src.ID)
	fmt.Printf("Type:  %s\n", src.Type)
	fmt.Printf("URL:   %s\n", src.URL)
	return nil
}

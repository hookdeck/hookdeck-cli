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

type sourceCreateCmd struct {
	cmd *cobra.Command

	name        string
	description string
	sourceType  string
	config      string
	configFile  string
	output      string

	sourceConfigFlags
}

func newSourceCreateCmd() *sourceCreateCmd {
	sc := &sourceCreateCmd{}

	sc.cmd = &cobra.Command{
		Use:   "create",
		Args:  validators.NoArgs,
		Short: ShortCreate(ResourceSource),
		Long: `Create a new source.

Requires --name and --type. Use --config or --config-file for authentication (e.g. webhook_secret, api_key).

Examples:
  hookdeck gateway source create --name my-webhook --type WEBHOOK
  hookdeck gateway source create --name stripe-prod --type STRIPE --config '{"webhook_secret":"whsec_xxx"}'`,
		PreRunE: sc.validateFlags,
		RunE:    sc.runSourceCreateCmd,
	}

	sc.cmd.Flags().StringVar(&sc.name, "name", "", "Source name (required)")
	sc.cmd.Flags().StringVar(&sc.description, "description", "", "Source description")
	sc.cmd.Flags().StringVar(&sc.sourceType, "type", "", "Source type (e.g. WEBHOOK, STRIPE) (required)")
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
	sc.cmd.Flags().StringVar(&sc.output, "output", "", "Output format (json)")

	sc.cmd.MarkFlagRequired("name")
	sc.cmd.MarkFlagRequired("type")

	return sc
}

func (sc *sourceCreateCmd) validateFlags(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
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

func (sc *sourceCreateCmd) runSourceCreateCmd(cmd *cobra.Command, args []string) error {
	client := Config.GetAPIClient()
	ctx := context.Background()

	config, err := buildSourceConfigFromFlags(sc.config, sc.configFile, &sc.sourceConfigFlags, sc.sourceType)
	if err != nil {
		return err
	}
	if config != nil {
		ensureSourceConfigAuthTypeForHTTP(config, sc.sourceType)
	}

	req := &hookdeck.SourceCreateRequest{
		Name: sc.name,
		Type: strings.ToUpper(sc.sourceType),
	}
	if sc.description != "" {
		req.Description = &sc.description
	}
	if len(config) > 0 {
		req.Config = config
	}

	src, err := client.CreateSource(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create source: %w", err)
	}

	if sc.output == "json" {
		jsonBytes, err := json.MarshalIndent(src, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal source to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	fmt.Printf(SuccessCheck + " Source created successfully\n\n")
	fmt.Printf("Source: %s (%s)\n", src.Name, src.ID)
	fmt.Printf("Type:  %s\n", src.Type)
	fmt.Printf("URL:   %s\n", src.URL)
	return nil
}

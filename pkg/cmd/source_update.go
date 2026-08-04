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

type sourceUpdateCmd struct {
	cmd *cobra.Command

	name        string
	description string
	sourceType  string
	config      string
	configFile  string
	output      string

	sourceConfigFlags
}

func newSourceUpdateCmd() *sourceUpdateCmd {
	sc := &sourceUpdateCmd{}

	sc.cmd = &cobra.Command{
		Use:   "update <source-id>",
		Args:  validators.ExactArgs(1),
		Short: ShortUpdate(ResourceSource),
		Long: LongUpdateIntro(ResourceSource) + `

Examples:
  hookdeck gateway source update src_abc123 --name new-name
  hookdeck gateway source update src_abc123 --description "Updated"
  hookdeck gateway source update src_abc123 --config '{"webhook_secret":"whsec_new"}'`,
		PreRunE: sc.validateFlags,
		RunE:    sc.runSourceUpdateCmd,
	}

	sc.cmd.Flags().StringVar(&sc.name, "name", "", "New source name")
	sc.cmd.Flags().StringVar(&sc.description, "description", "", "New source description")
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
	sc.cmd.Flags().StringVar(&sc.output, "output", "", "Output format (json)")

	return sc
}

// sourceUpdateRequestEmpty reports whether the update request has no fields set (all omitted).
// OpenAPI .plans/openapi-2025-07-01.json PUT /sources/{id} allows name, type, description, config.
func sourceUpdateRequestEmpty(req *hookdeck.SourceUpdateRequest) bool {
	return req.Name == "" && req.Description == nil && req.Type == "" && len(req.Config) == 0
}

func (sc *sourceUpdateCmd) validateFlags(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	if sc.config != "" && sc.configFile != "" {
		return fmt.Errorf("cannot use both --config and --config-file")
	}
	return nil
}

func (sc *sourceUpdateCmd) runSourceUpdateCmd(cmd *cobra.Command, args []string) error {
	sourceID := args[0]
	client := Config.GetAPIClient()
	ctx := context.Background()

	// Build update request from flags (only set non-zero values). Use SourceUpdateRequest so
	// omitted fields are not sent (PUT /sources/{id} has no required fields).
	req := &hookdeck.SourceUpdateRequest{}
	req.Name = sc.name
	if sc.description != "" {
		req.Description = &sc.description
	}
	if sc.sourceType != "" {
		req.Type = strings.ToUpper(sc.sourceType)
	}
	config, err := buildSourceConfigFromFlags(sc.config, sc.configFile, &sc.sourceConfigFlags, sc.sourceType)
	if err != nil {
		return err
	}
	if config != nil {
		ensureSourceConfigAuthTypeForHTTP(config, sc.sourceType)
	}
	if len(config) > 0 {
		req.Config = config
	}

	// Only send fields that were explicitly set. Spec: PUT /sources/{id} allows name, type, description, config.
	if sourceUpdateRequestEmpty(req) {
		return fmt.Errorf("no updates specified (set at least one of --name, --description, --type, or config flags)")
	}

	src, err := client.UpdateSource(ctx, sourceID, req)
	if err != nil {
		return fmt.Errorf("failed to update source: %w", err)
	}

	if sc.output == "json" {
		jsonBytes, err := json.MarshalIndent(src, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal source to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	fmt.Printf(SuccessCheck + " Source updated successfully\n\n")
	fmt.Printf("Source: %s (%s)\n", src.Name, src.ID)
	fmt.Printf("Type:  %s\n", src.Type)
	fmt.Printf("URL:   %s\n", src.URL)
	return nil
}

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

type destinationUpdateCmd struct {
	cmd *cobra.Command

	name        string
	description string
	destType    string
	url         string
	cliPath     string
	config      string
	configFile  string
	output      string

	destinationConfigFlags
}

func newDestinationUpdateCmd() *destinationUpdateCmd {
	dc := &destinationUpdateCmd{}

	dc.cmd = &cobra.Command{
		Use:   "update <destination-id>",
		Args:  validators.ExactArgs(1),
		Short: ShortUpdate(ResourceDestination),
		Long: LongUpdateIntro(ResourceDestination) + `

Examples:
  hookdeck gateway destination update des_abc123 --name new-name
  hookdeck gateway destination update des_abc123 --description "Updated"
  hookdeck gateway destination update des_abc123 --url https://api.example.com/new`,
		PreRunE: dc.validateFlags,
		RunE:    dc.runDestinationUpdateCmd,
	}

	dc.cmd.Flags().StringVar(&dc.name, "name", "", "New destination name")
	dc.cmd.Flags().StringVar(&dc.description, "description", "", "New destination description")
	dc.cmd.Flags().StringVar(&dc.destType, "type", "", "Destination type (HTTP, CLI, MOCK_API)")
	dc.cmd.Flags().StringVar(&dc.url, "url", "", "URL for HTTP destinations")
	dc.cmd.Flags().StringVar(&dc.cliPath, "cli-path", "", "Path for CLI destinations")
	dc.cmd.Flags().StringVar(&dc.config, "config", "", "JSON object for destination config (overrides individual flags if set)")
	dc.cmd.Flags().StringVar(&dc.configFile, "config-file", "", "Path to JSON file for destination config (overrides individual flags if set)")
	dc.cmd.Flags().StringVar(&dc.AuthMethod, "auth-method", "", "Auth method (hookdeck, bearer, basic, api_key, custom_signature)")
	dc.cmd.Flags().StringVar(&dc.BearerToken, "bearer-token", "", "Bearer token for destination auth")
	dc.cmd.Flags().StringVar(&dc.BasicAuthUser, "basic-auth-user", "", "Username for Basic auth")
	dc.cmd.Flags().StringVar(&dc.BasicAuthPass, "basic-auth-pass", "", "Password for Basic auth")
	dc.cmd.Flags().StringVar(&dc.APIKey, "api-key", "", "API key for destination auth")
	dc.cmd.Flags().StringVar(&dc.APIKeyHeader, "api-key-header", "", "Header/key name for API key")
	dc.cmd.Flags().StringVar(&dc.APIKeyTo, "api-key-to", "header", "Where to send API key (header or query)")
	dc.cmd.Flags().StringVar(&dc.CustomSignatureSecret, "custom-signature-secret", "", "Signing secret for custom signature")
	dc.cmd.Flags().StringVar(&dc.CustomSignatureKey, "custom-signature-key", "", "Key/header name for custom signature")
	dc.cmd.Flags().IntVar(&dc.RateLimit, "rate-limit", 0, "Rate limit (requests per period)")
	dc.cmd.Flags().StringVar(&dc.RateLimitPeriod, "rate-limit-period", "", "Rate limit period (second, minute, hour, concurrent)")
	dc.cmd.Flags().StringVar(&dc.HTTPMethod, "http-method", "", "HTTP method for HTTP destinations")
	dc.cmd.Flags().StringVar(&dc.output, "output", "", "Output format (json)")

	return dc
}

func destinationUpdateRequestEmpty(req *hookdeck.DestinationUpdateRequest) bool {
	return req.Name == "" && req.Description == nil && req.Type == "" && len(req.Config) == 0
}

func (dc *destinationUpdateCmd) validateFlags(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	if dc.config != "" && dc.configFile != "" {
		return fmt.Errorf("cannot use both --config and --config-file")
	}
	if dc.RateLimit > 0 && dc.RateLimitPeriod == "" {
		return fmt.Errorf("--rate-limit-period is required when --rate-limit is set")
	}
	return nil
}

func (dc *destinationUpdateCmd) runDestinationUpdateCmd(cmd *cobra.Command, args []string) error {
	destID := args[0]
	client := Config.GetAPIClient()
	ctx := context.Background()

	dc.destinationConfigFlags.URL = dc.url
	dc.destinationConfigFlags.CliPath = dc.cliPath

	req := &hookdeck.DestinationUpdateRequest{}
	req.Name = dc.name
	if dc.description != "" {
		req.Description = &dc.description
	}
	if dc.destType != "" {
		req.Type = strings.ToUpper(dc.destType)
	}
	config, err := buildDestinationConfigFromFlags(dc.config, dc.configFile, dc.destType, &dc.destinationConfigFlags)
	if err != nil {
		return err
	}
	if len(config) > 0 {
		req.Config = config
	}

	if destinationUpdateRequestEmpty(req) {
		return fmt.Errorf("no updates specified (set at least one of --name, --description, --type, or config flags)")
	}

	dst, err := client.UpdateDestination(ctx, destID, req)
	if err != nil {
		return fmt.Errorf("failed to update destination: %w", err)
	}

	if dc.output == "json" {
		jsonBytes, err := json.MarshalIndent(dst, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal destination to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	fmt.Printf("✔ Destination updated successfully\n\n")
	fmt.Printf("Destination: %s (%s)\n", dst.Name, dst.ID)
	fmt.Printf("Type: %s\n", dst.Type)
	if u := dst.GetHTTPURL(); u != nil {
		fmt.Printf("URL: %s\n", *u)
	}
	return nil
}

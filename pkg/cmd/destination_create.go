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

type destinationCreateCmd struct {
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

func newDestinationCreateCmd() *destinationCreateCmd {
	dc := &destinationCreateCmd{}

	dc.cmd = &cobra.Command{
		Use:   "create",
		Args:  validators.NoArgs,
		Short: ShortCreate(ResourceDestination),
		Long: `Create a new destination.

Requires --name and --type. For HTTP destinations, --url is required. Use --config or --config-file for auth and rate limiting.

Examples:
  hookdeck gateway destination create --name my-api --type HTTP --url https://api.example.com/webhooks
  hookdeck gateway destination create --name local-cli --type CLI --cli-path /webhooks
  hookdeck gateway destination create --name my-api --type HTTP --url https://api.example.com --bearer-token token123`,
		PreRunE: dc.validateFlags,
		RunE:    dc.runDestinationCreateCmd,
	}

	dc.cmd.Flags().StringVar(&dc.name, "name", "", "Destination name (required)")
	dc.cmd.Flags().StringVar(&dc.description, "description", "", "Destination description")
	dc.cmd.Flags().StringVar(&dc.destType, "type", "", "Destination type (HTTP, CLI, MOCK_API) (required)")
	dc.cmd.Flags().StringVar(&dc.url, "url", "", "URL for HTTP destinations (required for type HTTP)")
	dc.cmd.Flags().StringVar(&dc.cliPath, "cli-path", "/", "Path for CLI destinations")
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
	dc.cmd.Flags().StringVar(&dc.HTTPMethod, "http-method", "", "HTTP method for HTTP destinations (GET, POST, PUT, PATCH, DELETE)")
	dc.cmd.Flags().StringVar(&dc.output, "output", "", "Output format (json)")

	dc.cmd.MarkFlagRequired("name")
	dc.cmd.MarkFlagRequired("type")

	return dc
}

func (dc *destinationCreateCmd) validateFlags(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	if dc.config != "" && dc.configFile != "" {
		return fmt.Errorf("cannot use both --config and --config-file")
	}
	t := strings.ToUpper(dc.destType)
	if t == "HTTP" && dc.url == "" && dc.config == "" && dc.configFile == "" {
		return fmt.Errorf("--url is required for HTTP destinations")
	}
	if dc.RateLimit > 0 && dc.RateLimitPeriod == "" {
		return fmt.Errorf("--rate-limit-period is required when --rate-limit is set")
	}
	return nil
}

func (dc *destinationCreateCmd) runDestinationCreateCmd(cmd *cobra.Command, args []string) error {
	client := Config.GetAPIClient()
	ctx := context.Background()

	// Sync url/cliPath into flags for buildDestinationConfigFromIndividualFlags when not using --config
	dc.destinationConfigFlags.URL = dc.url
	dc.destinationConfigFlags.CliPath = dc.cliPath

	config, err := buildDestinationConfigFromFlags(dc.config, dc.configFile, dc.destType, &dc.destinationConfigFlags)
	if err != nil {
		return err
	}

	// For HTTP/CLI, ensure url/path in config when using individual flags
	t := strings.ToUpper(dc.destType)
	if config == nil {
		config = make(map[string]interface{})
	}
	if t == "HTTP" && dc.url != "" {
		config["url"] = dc.url
	}
	if t == "CLI" {
		path := dc.cliPath
		if path == "" {
			path = "/"
		}
		config["path"] = path
	}

	req := &hookdeck.DestinationCreateRequest{
		Name: dc.name,
		Type: t,
	}
	if dc.description != "" {
		req.Description = &dc.description
	}
	if len(config) > 0 {
		req.Config = config
	}

	dst, err := client.CreateDestination(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}

	if dc.output == "json" {
		jsonBytes, err := json.MarshalIndent(dst, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal destination to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	fmt.Printf(SuccessCheck + " Destination created successfully\n\n")
	fmt.Printf("Destination: %s (%s)\n", dst.Name, dst.ID)
	fmt.Printf("Type: %s\n", dst.Type)
	if u := dst.GetHTTPURL(); u != nil {
		fmt.Printf("URL: %s\n", *u)
	}
	return nil
}

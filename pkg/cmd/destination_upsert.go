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
	dc.cmd.Flags().BoolVar(&dc.dryRun, "dry-run", false, "Preview changes without applying")
	dc.cmd.Flags().StringVar(&dc.output, "output", "", "Output format (json)")

	return dc
}

func (dc *destinationUpsertCmd) validateFlags(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	dc.name = args[0]
	if dc.config != "" && dc.configFile != "" {
		return fmt.Errorf("cannot use both --config and --config-file")
	}
	if dc.RateLimit > 0 && dc.RateLimitPeriod == "" {
		return fmt.Errorf("--rate-limit-period is required when --rate-limit is set")
	}
	return nil
}

func (dc *destinationUpsertCmd) runDestinationUpsertCmd(cmd *cobra.Command, args []string) error {
	client := Config.GetAPIClient()
	ctx := context.Background()

	dc.destinationConfigFlags.URL = dc.url
	dc.destinationConfigFlags.CliPath = dc.cliPath

	config, err := buildDestinationConfigFromFlags(dc.config, dc.configFile, dc.destType, &dc.destinationConfigFlags)
	if err != nil {
		return err
	}

	t := strings.ToUpper(dc.destType)
	if config == nil {
		config = make(map[string]interface{})
	}
	if t == "HTTP" && dc.url != "" {
		config["url"] = dc.url
	}
	if t == "CLI" && dc.cliPath != "" {
		config["path"] = dc.cliPath
	}

	req := &hookdeck.DestinationCreateRequest{
		Name: dc.name,
	}
	if dc.description != "" {
		req.Description = &dc.description
	}
	if t != "" {
		req.Type = t
	}
	if len(config) > 0 {
		req.Config = config
	}

	// API requires config on PUT. When doing partial update (e.g. only --description), fetch existing and merge.
	if req.Config == nil || len(req.Config) == 0 {
		params := map[string]string{"name": dc.name}
		listResp, err := client.ListDestinations(ctx, params)
		if err == nil && listResp.Models != nil && len(listResp.Models) > 0 {
			existing, err := client.GetDestination(ctx, listResp.Models[0].ID, nil)
			if err == nil && existing.Config != nil {
				req.Config = existing.Config
				if req.Type == "" {
					req.Type = existing.Type
				}
			}
		}
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

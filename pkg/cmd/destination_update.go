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
	addDestinationDeliveryPolicyFlags(dc.cmd, &dc.destinationConfigFlags)
	dc.cmd.Flags().StringVar(&dc.HTTPMethod, "http-method", "", "HTTP method for HTTP destinations")
	dc.cmd.Flags().StringVar(&dc.output, "output", "", "Output format (json)")

	return dc
}

func destinationUpdateRequestEmpty(req *hookdeck.DestinationUpdateRequest) bool {
	return req.Name == "" && req.Description == nil && req.Type == "" && len(req.Config) == 0
}

func (dc *destinationUpdateCmd) validateFlags(cmd *cobra.Command, args []string) error {
	// An explicitly empty value for a secret or identity flag cannot mean
	// anything, and is almost always an unexported shell variable (#335).
	if err := rejectEmptyFlags(cmd); err != nil {
		return err
	}

	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	if dc.config != "" && dc.configFile != "" {
		return fmt.Errorf("cannot use both --config and --config-file")
	}
	// --config / --config-file take precedence: buildDestinationConfigFromFlags
	// returns their JSON and never looks at the individual flags. Validating
	// those flags here anyway rejected commands over a value that would have
	// been ignored.
	if dc.config != "" || dc.configFile != "" {
		return nil
	}
	return dc.destinationConfigFlags.validateDeliveryPolicyFlags("")
}

func (dc *destinationUpdateCmd) runDestinationUpdateCmd(cmd *cobra.Command, args []string) error {
	destID := args[0]
	client := Config.GetAPIClient()
	ctx := context.Background()

	req, err := dc.buildUpdateRequest(ctx, client, destID)
	if err != nil {
		return err
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

	fmt.Printf(SuccessCheck + " Destination updated successfully\n\n")
	fmt.Printf("Destination: %s (%s)\n", dst.Name, dst.ID)
	fmt.Printf("Type: %s\n", dst.Type)
	if u := dst.GetHTTPURL(); u != nil {
		fmt.Printf("URL: %s\n", *u)
	}
	return nil
}

// buildUpdateRequest assembles the PUT body, consulting the stored destination
// where a flag alone cannot answer the question. Separated from the command so
// the guards it applies are reachable from a test with a real HTTP client.
func (dc *destinationUpdateCmd) buildUpdateRequest(ctx context.Context, client *hookdeck.Client, destID string) (*hookdeck.DestinationUpdateRequest, error) {
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
	// The stored destination answers two questions below: what type it is, so
	// the CLI delivery-policy guard can run when --type is omitted, and what
	// delivery-group overrides it holds, so a bare groups object does not
	// destroy them. Fetch it at most once, and only when it is needed.
	var (
		existingDest    *hookdeck.Destination
		fetchedExisting bool
	)
	lookupExisting := func() (*hookdeck.Destination, error) {
		if fetchedExisting {
			return existingDest, nil
		}
		found, err := client.GetDestination(ctx, destID, nil)
		if err != nil {
			return nil, err
		}
		existingDest, fetchedExisting = found, true
		return found, nil
	}

	// --type is normally omitted on update, so the delivery-policy guard has to
	// resolve the stored type or it never fires for the common invocation.
	policyType, err := destinationTypeForPolicyCheck(
		dc.destType,
		dc.config != "" || dc.configFile != "",
		&dc.destinationConfigFlags,
		lookupExisting,
	)
	if err != nil {
		return nil, err
	}

	config, err := buildDestinationConfigFromFlags(dc.config, dc.configFile, dc.destType, &dc.destinationConfigFlags)
	if err != nil {
		return nil, err
	}
	if err := rejectDeliveryPolicyInConfigForCLI(policyType, config, ""); err != nil {
		return nil, err
	}
	// update is a PUT and the API replaces delivery_policy.groups wholesale, so
	// "just bump the group rate" was destroying the stored overrides here just
	// as it was on upsert (#393).
	if err := preserveStoredDeliveryGroupOverrides(destID, config, lookupExisting); err != nil {
		return nil, err
	}
	if len(config) > 0 {
		req.Config = config
	}

	if destinationUpdateRequestEmpty(req) {
		return nil, fmt.Errorf("no updates specified (set at least one of --name, --description, --type, or config flags)")
	}

	return req, nil
}

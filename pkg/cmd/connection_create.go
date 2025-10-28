package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/cmd/sources"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type connectionCreateCmd struct {
	cmd *cobra.Command

	// Command flags
	output string

	// Connection flags
	name        string
	description string

	// Source flags (inline creation)
	sourceName        string
	sourceType        string
	sourceDescription string

	// Universal source authentication flags
	SourceWebhookSecret string
	SourceAPIKey        string
	SourceBasicAuthUser string
	SourceBasicAuthPass string
	SourceHMACSecret    string
	SourceHMACAlgo      string

	// JSON config fallback
	SourceConfig     string
	SourceConfigFile string

	// Destination flags (inline creation)
	destinationName        string
	destinationType        string
	destinationDescription string
	destinationURL         string
	destinationCliPath     string

	// Destination authentication flags
	DestinationAuthMethod        string
	DestinationBearerToken       string
	DestinationBasicAuthUser     string
	DestinationBasicAuthPass     string
	DestinationAPIKey            string
	DestinationAPIKeyHeader      string
	DestinationHMACSecret        string
	DestinationHMACAlgo          string
	DestinationHMACHeader        string
	DestinationOauthClientID     string
	DestinationOauthClientSecret string
	DestinationOauthTokenURL     string

	// Reference existing resources
	sourceID      string
	destinationID string
}

func newConnectionCreateCmd() *connectionCreateCmd {
	cc := &connectionCreateCmd{}

	cc.cmd = &cobra.Command{
		Use:   "create",
		Args:  validators.NoArgs,
		Short: "Create a new connection",
		Long: `Create a connection between a source and destination.

You can either reference existing resources by ID or create them inline.

Examples:
  # Create with inline source and destination
  hookdeck connection create \
    --name "test-webhooks-to-local" \
    --source-type WEBHOOK --source-name "test-webhooks" \
    --destination-type CLI --destination-name "local-dev"

  # Create with existing resources
  hookdeck connection create \
    --name "github-to-api" \
    --source-id src_abc123 \
    --destination-id dst_def456`,
		PreRunE: cc.validateFlags,
		RunE:    cc.runConnectionCreateCmd,
	}

	// Connection flags
	cc.cmd.Flags().StringVar(&cc.name, "name", "", "Connection name (required)")
	cc.cmd.Flags().StringVar(&cc.description, "description", "", "Connection description")

	// Source inline creation flags
	cc.cmd.Flags().StringVar(&cc.sourceName, "source-name", "", "Source name for inline creation")
	cc.cmd.Flags().StringVar(&cc.sourceType, "source-type", "", "Source type (WEBHOOK, STRIPE, etc.)")
	cc.cmd.Flags().StringVar(&cc.sourceDescription, "source-description", "", "Source description")

	// Universal source authentication flags
	cc.cmd.Flags().StringVar(&cc.SourceWebhookSecret, "source-webhook-secret", "", "Webhook secret for source verification (e.g., Stripe)")
	cc.cmd.Flags().StringVar(&cc.SourceAPIKey, "source-api-key", "", "API key for source authentication")
	cc.cmd.Flags().StringVar(&cc.SourceBasicAuthUser, "source-basic-auth-user", "", "Username for Basic authentication")
	cc.cmd.Flags().StringVar(&cc.SourceBasicAuthPass, "source-basic-auth-pass", "", "Password for Basic authentication")
	cc.cmd.Flags().StringVar(&cc.SourceHMACSecret, "source-hmac-secret", "", "HMAC secret for signature verification")
	cc.cmd.Flags().StringVar(&cc.SourceHMACAlgo, "source-hmac-algo", "", "HMAC algorithm (SHA256, etc.)")

	// JSON config fallback
	cc.cmd.Flags().StringVar(&cc.SourceConfig, "source-config", "", "JSON string for source authentication config")
	cc.cmd.Flags().StringVar(&cc.SourceConfigFile, "source-config-file", "", "Path to a JSON file for source authentication config")

	// Destination inline creation flags
	cc.cmd.Flags().StringVar(&cc.destinationName, "destination-name", "", "Destination name for inline creation")
	cc.cmd.Flags().StringVar(&cc.destinationType, "destination-type", "", "Destination type (CLI, HTTP, MOCK)")
	cc.cmd.Flags().StringVar(&cc.destinationDescription, "destination-description", "", "Destination description")
	cc.cmd.Flags().StringVar(&cc.destinationURL, "destination-url", "", "URL for HTTP destinations")
	cc.cmd.Flags().StringVar(&cc.destinationCliPath, "destination-cli-path", "/", "CLI path for CLI destinations (default: /)")

	// Destination authentication flags
	cc.cmd.Flags().StringVar(&cc.DestinationAuthMethod, "destination-auth-method", "", "Authentication method for HTTP destinations (e.g., bearer, basic, api_key, hmac, oauth2)")
	cc.cmd.Flags().StringVar(&cc.DestinationBearerToken, "destination-bearer-token", "", "Bearer token for destination authentication")
	cc.cmd.Flags().StringVar(&cc.DestinationBasicAuthUser, "destination-basic-auth-user", "", "Username for destination Basic authentication")
	cc.cmd.Flags().StringVar(&cc.DestinationBasicAuthPass, "destination-basic-auth-pass", "", "Password for destination Basic authentication")
	cc.cmd.Flags().StringVar(&cc.DestinationAPIKey, "destination-api-key", "", "API key for destination authentication")
	cc.cmd.Flags().StringVar(&cc.DestinationAPIKeyHeader, "destination-api-key-header", "Authorization", "Header to use for API key authentication")
	cc.cmd.Flags().StringVar(&cc.DestinationHMACSecret, "destination-hmac-secret", "", "HMAC secret for destination signature verification")
	cc.cmd.Flags().StringVar(&cc.DestinationHMACAlgo, "destination-hmac-algo", "sha256", "HMAC algorithm for destination signature")
	cc.cmd.Flags().StringVar(&cc.DestinationHMACHeader, "destination-hmac-header", "X-Signature-256", "Header to use for HMAC signature")
	cc.cmd.Flags().StringVar(&cc.DestinationOauthClientID, "destination-oauth-client-id", "", "OAuth2 client ID for destination authentication")
	cc.cmd.Flags().StringVar(&cc.DestinationOauthClientSecret, "destination-oauth-client-secret", "", "OAuth2 client secret for destination authentication")
	cc.cmd.Flags().StringVar(&cc.DestinationOauthTokenURL, "destination-oauth-token-url", "", "OAuth2 token URL for destination authentication")

	// Reference existing resources
	cc.cmd.Flags().StringVar(&cc.sourceID, "source-id", "", "Use existing source by ID")
	cc.cmd.Flags().StringVar(&cc.destinationID, "destination-id", "", "Use existing destination by ID")

	// Output flags
	cc.cmd.Flags().StringVar(&cc.output, "output", "", "Output format (json)")

	cc.cmd.MarkFlagRequired("name")

	return cc
}

func (cc *connectionCreateCmd) validateFlags(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	// Check for inline vs reference mode for source
	hasInlineSource := cc.sourceName != "" || cc.sourceType != ""

	if hasInlineSource && cc.sourceID != "" {
		return fmt.Errorf("cannot specify both inline source creation (--source-name, --source-type) and --source-id")
	}
	if !hasInlineSource && cc.sourceID == "" {
		return fmt.Errorf("must specify either source creation flags (--source-name and --source-type) or --source-id")
	}

	// Validate inline source creation
	if hasInlineSource {
		if cc.sourceName == "" {
			return fmt.Errorf("--source-name is required when creating a source inline")
		}
		if cc.sourceType == "" {
			return fmt.Errorf("--source-type is required when creating a source inline")
		}
	}

	// Check for inline vs reference mode for destination
	hasInlineDestination := cc.destinationName != "" || cc.destinationType != ""

	if hasInlineDestination && cc.destinationID != "" {
		return fmt.Errorf("cannot specify both inline destination creation (--destination-name, --destination-type) and --destination-id")
	}
	if !hasInlineDestination && cc.destinationID == "" {
		return fmt.Errorf("must specify either destination creation flags (--destination-name and --destination-type) or --destination-id")
	}

	// Validate inline destination creation
	if hasInlineDestination {
		if cc.destinationName == "" {
			return fmt.Errorf("--destination-name is required when creating a destination inline")
		}
		if cc.destinationType == "" {
			return fmt.Errorf("--destination-type is required when creating a destination inline")
		}
	}

	// Validate source authentication flags based on source type
	if hasInlineSource && cc.SourceConfig == "" && cc.SourceConfigFile == "" {
		sourceTypes, err := sources.FetchSourceTypes()
		if err != nil {
			// We can't validate, so we'll just warn and let the API handle it
			fmt.Printf("Warning: could not fetch source types for validation: %v\n", err)
			return nil
		}

		sourceType, ok := sourceTypes[strings.ToUpper(cc.sourceType)]
		if !ok {
			// This is an unknown source type, let the API validate it
			return nil
		}

		switch sourceType.AuthScheme {
		case "webhook_secret":
			if cc.SourceWebhookSecret == "" {
				return fmt.Errorf("error: --source-webhook-secret is required for source type %s", cc.sourceType)
			}
		case "api_key":
			if cc.SourceAPIKey == "" {
				return fmt.Errorf("error: --source-api-key is required for source type %s", cc.sourceType)
			}
		case "basic_auth":
			if cc.SourceBasicAuthUser == "" || cc.SourceBasicAuthPass == "" {
				return fmt.Errorf("error: --source-basic-auth-user and --source-basic-auth-pass are required for source type %s", cc.sourceType)
			}
		case "hmac":
			if cc.SourceHMACSecret == "" {
				return fmt.Errorf("error: --source-hmac-secret is required for source type %s", cc.sourceType)
			}
		}
	}

	return nil
}

func (cc *connectionCreateCmd) runConnectionCreateCmd(cmd *cobra.Command, args []string) error {
	client := Config.GetAPIClient()

	req := &hookdeck.ConnectionCreateRequest{
		Name: &cc.name,
	}
	if cc.description != "" {
		req.Description = &cc.description
	}

	// Handle Source
	if cc.sourceID != "" {
		req.SourceID = &cc.sourceID
	} else {
		if cc.output != "json" {
			fmt.Printf("Building source '%s' (%s)...\n", cc.sourceName, cc.sourceType)
		}
		sourceInput, err := cc.buildSourceInput()
		if err != nil {
			return err
		}
		req.Source = sourceInput
	}

	// Handle Destination
	if cc.destinationID != "" {
		req.DestinationID = &cc.destinationID
	} else {
		if cc.output != "json" {
			fmt.Printf("Building destination '%s' (%s)...\n", cc.destinationName, cc.destinationType)
		}
		destinationInput, err := cc.buildDestinationInput()
		if err != nil {
			return err
		}
		req.Destination = destinationInput
	}

	if cc.output != "json" {
		fmt.Printf("Creating connection '%s'...\n", cc.name)
	}

	// Single API call to create the connection
	connection, err := client.CreateConnection(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to create connection: %w", err)
	}

	// Display results
	if cc.output == "json" {
		jsonBytes, err := json.MarshalIndent(connection, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal connection to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
	} else {
		fmt.Printf("\n✓ Connection created successfully\n\n")
		if connection.Name != nil {
			fmt.Printf("Connection:  %s (%s)\n", *connection.Name, connection.ID)
		} else {
			fmt.Printf("Connection:  (unnamed)\n")
		}

		if connection.Source != nil {
			fmt.Printf("Source:      %s (%s)\n", connection.Source.Name, connection.Source.ID)
			fmt.Printf("Source URL:  %s\n", connection.Source.URL)
		}

		if connection.Destination != nil {
			fmt.Printf("Destination: %s (%s)\n", connection.Destination.Name, connection.Destination.ID)
		}
	}

	return nil
}

func (cc *connectionCreateCmd) buildSourceInput() (*hookdeck.SourceCreateInput, error) {
	var description *string
	if cc.sourceDescription != "" {
		description = &cc.sourceDescription
	}

	sourceConfig, err := cc.buildSourceConfig()
	if err != nil {
		return nil, fmt.Errorf("error building source config: %w", err)
	}

	return &hookdeck.SourceCreateInput{
		Name:        cc.sourceName,
		Description: description,
		Type:        strings.ToUpper(cc.sourceType),
		Config:      sourceConfig,
	}, nil
}

func (cc *connectionCreateCmd) buildDestinationInput() (*hookdeck.DestinationCreateInput, error) {
	var description *string
	if cc.destinationDescription != "" {
		description = &cc.destinationDescription
	}

	destinationConfig, err := cc.buildDestinationConfig()
	if err != nil {
		return nil, fmt.Errorf("error building destination config: %w", err)
	}

	input := &hookdeck.DestinationCreateInput{
		Name:        cc.destinationName,
		Description: description,
		Type:        strings.ToUpper(cc.destinationType),
	}

	// Type is not part of the main struct, but part of the config
	// We need to handle this based on the API spec
	switch strings.ToUpper(cc.destinationType) {
	case "HTTP":
		if cc.destinationURL == "" {
			return nil, fmt.Errorf("--destination-url is required for HTTP destinations")
		}
		destinationConfig["url"] = cc.destinationURL
	case "CLI":
		destinationConfig["path"] = cc.destinationCliPath
	case "MOCK_API":
		// No extra fields needed for MOCK_API
	default:
		return nil, fmt.Errorf("unsupported destination type: %s (supported: CLI, HTTP, MOCK_API)", cc.destinationType)
	}
	input.Config = destinationConfig

	return input, nil
}

func (cc *connectionCreateCmd) buildDestinationConfig() (map[string]interface{}, error) {
	config := make(map[string]interface{})

	authConfig := make(map[string]interface{})

	switch cc.DestinationAuthMethod {
	case "bearer":
		if cc.DestinationBearerToken == "" {
			return nil, fmt.Errorf("--destination-bearer-token is required for bearer auth method")
		}
		authConfig["type"] = "bearer"
		authConfig["config"] = map[string]string{"token": cc.DestinationBearerToken}
	case "basic":
		if cc.DestinationBasicAuthUser == "" || cc.DestinationBasicAuthPass == "" {
			return nil, fmt.Errorf("--destination-basic-auth-user and --destination-basic-auth-pass are required for basic auth method")
		}
		authConfig["type"] = "basic_auth"
		authConfig["config"] = map[string]string{
			"username": cc.DestinationBasicAuthUser,
			"password": cc.DestinationBasicAuthPass,
		}
	case "api_key":
		if cc.DestinationAPIKey == "" {
			return nil, fmt.Errorf("--destination-api-key is required for api_key auth method")
		}
		authConfig["type"] = "api_key"
		authConfig["config"] = map[string]string{
			"key":    cc.DestinationAPIKey,
			"header": cc.DestinationAPIKeyHeader,
		}
	case "hmac":
		if cc.DestinationHMACSecret == "" {
			return nil, fmt.Errorf("--destination-hmac-secret is required for hmac auth method")
		}
		authConfig["type"] = "hmac"
		authConfig["config"] = map[string]string{
			"secret":    cc.DestinationHMACSecret,
			"algorithm": cc.DestinationHMACAlgo,
			"header":    cc.DestinationHMACHeader,
		}
	case "oauth2":
		if cc.DestinationOauthClientID == "" || cc.DestinationOauthClientSecret == "" || cc.DestinationOauthTokenURL == "" {
			return nil, fmt.Errorf("--destination-oauth-client-id, --destination-oauth-client-secret, and --destination-oauth-token-url are required for oauth2 auth method")
		}
		authConfig["type"] = "oauth2"
		authConfig["config"] = map[string]string{
			"client_id":     cc.DestinationOauthClientID,
			"client_secret": cc.DestinationOauthClientSecret,
			"token_url":     cc.DestinationOauthTokenURL,
		}
	case "":
		// No auth method specified
	default:
		return nil, fmt.Errorf("unsupported destination authentication method: %s", cc.DestinationAuthMethod)
	}

	if len(authConfig) > 0 {
		config["auth_method"] = authConfig
	}

	if len(config) == 0 {
		return make(map[string]interface{}), nil
	}

	return config, nil
}

func (cc *connectionCreateCmd) buildSourceConfig() (map[string]interface{}, error) {
	// Handle JSON config first, as it overrides individual flags
	if cc.SourceConfig != "" {
		var config map[string]interface{}
		if err := json.Unmarshal([]byte(cc.SourceConfig), &config); err != nil {
			return nil, fmt.Errorf("invalid JSON in --source-config: %w", err)
		}
		return config, nil
	}
	if cc.SourceConfigFile != "" {
		data, err := os.ReadFile(cc.SourceConfigFile)
		if err != nil {
			return nil, fmt.Errorf("could not read --source-config-file: %w", err)
		}
		var config map[string]interface{}
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("invalid JSON in --source-config-file: %w", err)
		}
		return config, nil
	}

	// Build config from individual flags
	config := make(map[string]interface{})
	if cc.SourceWebhookSecret != "" {
		config["webhook_secret"] = cc.SourceWebhookSecret
	}
	if cc.SourceAPIKey != "" {
		config["api_key"] = cc.SourceAPIKey
	}
	if cc.SourceBasicAuthUser != "" || cc.SourceBasicAuthPass != "" {
		config["basic_auth"] = map[string]string{
			"username": cc.SourceBasicAuthUser,
			"password": cc.SourceBasicAuthPass,
		}
	}
	if cc.SourceHMACSecret != "" {
		hmacConfig := map[string]string{"secret": cc.SourceHMACSecret}
		if cc.SourceHMACAlgo != "" {
			hmacConfig["algorithm"] = cc.SourceHMACAlgo
		}
		config["hmac"] = hmacConfig
	}

	if len(config) == 0 {
		return make(map[string]interface{}), nil
	}

	return config, nil
}

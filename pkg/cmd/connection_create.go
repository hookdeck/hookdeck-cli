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
	destinationCliPath     string

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
	cc.cmd.Flags().StringVar(&cc.destinationCliPath, "destination-cli-path", "/", "CLI path for CLI destinations (default: /)")

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

	var sourceID, destinationID string
	var sourceName, destinationName string

	// Create source if needed
	if cc.sourceID != "" {
		sourceID = cc.sourceID
		// We'll get the name from the API response later
	} else {
		if cc.output != "json" {
			fmt.Printf("Creating source '%s' (%s)...\n", cc.sourceName, cc.sourceType)
		}

		var description *string
		if cc.sourceDescription != "" {
			description = &cc.sourceDescription
		}

		sourceConfig, err := cc.buildSourceConfig()
		if err != nil {
			return fmt.Errorf("error building source config: %w", err)
		}

		source, err := client.CreateSource(context.Background(), &hookdeck.SourceCreateRequest{
			Name:        cc.sourceName,
			Description: description,
			Config:      sourceConfig,
		})
		if err != nil {
			return fmt.Errorf("failed to create source: %w", err)
		}
		sourceID = source.ID
		sourceName = source.Name

		if cc.output != "json" {
			fmt.Printf("✓ Source created: %s (%s)\n", source.Name, source.ID)
			fmt.Printf("  Source URL: %s\n", source.URL)
		}
	}

	// Create destination if needed
	if cc.destinationID != "" {
		destinationID = cc.destinationID
		// We'll get the name from the API response later
	} else {
		if cc.output != "json" {
			fmt.Printf("Creating destination '%s' (%s)...\n", cc.destinationName, cc.destinationType)
		}

		var destinationReq *hookdeck.DestinationCreateRequest

		var description *string
		if cc.destinationDescription != "" {
			description = &cc.destinationDescription
		}

		switch cc.destinationType {
		case "CLI":
			var cliPath *string
			if cc.destinationCliPath != "" {
				cliPath = &cc.destinationCliPath
			}
			destinationReq = &hookdeck.DestinationCreateRequest{
				Name:        cc.destinationName,
				Description: description,
				CliPath:     cliPath,
			}
		case "HTTP":
			return fmt.Errorf("HTTP destination type requires --destination-url flag (not yet implemented)")
		case "MOCK":
			destinationReq = &hookdeck.DestinationCreateRequest{
				Name:        cc.destinationName,
				Description: description,
			}
		default:
			return fmt.Errorf("unsupported destination type: %s (supported: CLI, HTTP, MOCK)", cc.destinationType)
		}

		dest, err := client.CreateDestination(context.Background(), destinationReq)
		if err != nil {
			return fmt.Errorf("failed to create destination: %w", err)
		}
		destinationID = dest.ID
		destinationName = dest.Name

		if cc.output != "json" {
			fmt.Printf("✓ Destination created: %s (%s)\n", dest.Name, dest.ID)
		}
	}

	// Create connection
	if cc.output != "json" {
		fmt.Printf("Creating connection '%s'...\n", cc.name)
	}

	var connName *string
	if cc.name != "" {
		connName = &cc.name
	}

	var connDescription *string
	if cc.description != "" {
		connDescription = &cc.description
	}

	connection, err := client.CreateConnection(context.Background(), &hookdeck.ConnectionCreateRequest{
		Name:          connName,
		Description:   connDescription,
		SourceID:      &sourceID,
		DestinationID: &destinationID,
	})
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
			fmt.Printf("Connection:  %s\n", *connection.Name)
		} else {
			fmt.Printf("Connection:  (unnamed)\n")
		}

		// Get source name if we used existing source
		if sourceName == "" && connection.Source != nil {
			sourceName = connection.Source.Name
		}
		if connection.Source != nil {
			fmt.Printf("Source:      %s (%s)\n", sourceName, connection.Source.ID)
			fmt.Printf("Source URL:  %s\n", connection.Source.URL)
		}

		if cc.sourceType != "" {
			fmt.Printf("Source Type: %s\n", cc.sourceType)
		}

		// Get destination name if we used existing destination
		if destinationName == "" && connection.Destination != nil {
			destinationName = connection.Destination.Name
		}
		if connection.Destination != nil {
			fmt.Printf("Destination: %s (%s)\n", destinationName, connection.Destination.ID)
		}
	}

	return nil
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
		return nil, nil // No config provided
	}

	return config, nil
}

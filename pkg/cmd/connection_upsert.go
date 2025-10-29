package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

type connectionUpsertCmd struct {
	*connectionCreateCmd // Embed create command to reuse all flags and methods
	dryRun               bool
}

func newConnectionUpsertCmd() *connectionUpsertCmd {
	cu := &connectionUpsertCmd{
		connectionCreateCmd: &connectionCreateCmd{},
	}

	cu.cmd = &cobra.Command{
		Use:   "upsert <name>",
		Args:  cobra.ExactArgs(1),
		Short: "Create or update a connection by name",
		Long: `Create a new connection or update an existing one using name as the unique identifier.

This command is idempotent - it can be safely run multiple times with the same arguments.

When the connection doesn't exist:
  - Creates a new connection with the provided properties
  - Requires source and destination to be specified

When the connection exists:
  - Updates the connection with the provided properties
  - Only updates properties that are explicitly provided
  - Preserves existing properties that aren't specified

Use --dry-run to preview changes without applying them.

Examples:
  # Create or update a connection with inline source and destination
  hookdeck connection upsert "my-connection" \
    --source-name "stripe-prod" --source-type STRIPE \
    --destination-name "my-api" --destination-type HTTP --destination-url https://api.example.com

  # Update just the rate limit on an existing connection
  hookdeck connection upsert my-connection \
    --destination-rate-limit 100 --destination-rate-limit-period minute

  # Preview changes without applying them
  hookdeck connection upsert my-connection \
    --destination-rate-limit 200 --destination-rate-limit-period hour \
    --dry-run`,
		PreRunE: cu.validateUpsertFlags,
		RunE:    cu.runConnectionUpsertCmd,
	}

	// Reuse all flags from create command (name is now a positional argument)
	cu.cmd.Flags().StringVar(&cu.description, "description", "", "Connection description")

	// Source inline creation flags
	cu.cmd.Flags().StringVar(&cu.sourceName, "source-name", "", "Source name for inline creation")
	cu.cmd.Flags().StringVar(&cu.sourceType, "source-type", "", "Source type (WEBHOOK, STRIPE, etc.)")
	cu.cmd.Flags().StringVar(&cu.sourceDescription, "source-description", "", "Source description")

	// Universal source authentication flags
	cu.cmd.Flags().StringVar(&cu.SourceWebhookSecret, "source-webhook-secret", "", "Webhook secret for source verification (e.g., Stripe)")
	cu.cmd.Flags().StringVar(&cu.SourceAPIKey, "source-api-key", "", "API key for source authentication")
	cu.cmd.Flags().StringVar(&cu.SourceBasicAuthUser, "source-basic-auth-user", "", "Username for Basic authentication")
	cu.cmd.Flags().StringVar(&cu.SourceBasicAuthPass, "source-basic-auth-pass", "", "Password for Basic authentication")
	cu.cmd.Flags().StringVar(&cu.SourceHMACSecret, "source-hmac-secret", "", "HMAC secret for signature verification")
	cu.cmd.Flags().StringVar(&cu.SourceHMACAlgo, "source-hmac-algo", "", "HMAC algorithm (SHA256, etc.)")

	// JSON config fallback
	cu.cmd.Flags().StringVar(&cu.SourceConfig, "source-config", "", "JSON string for source authentication config")
	cu.cmd.Flags().StringVar(&cu.SourceConfigFile, "source-config-file", "", "Path to a JSON file for source authentication config")

	// Destination inline creation flags
	cu.cmd.Flags().StringVar(&cu.destinationName, "destination-name", "", "Destination name for inline creation")
	cu.cmd.Flags().StringVar(&cu.destinationType, "destination-type", "", "Destination type (CLI, HTTP, MOCK)")
	cu.cmd.Flags().StringVar(&cu.destinationDescription, "destination-description", "", "Destination description")
	cu.cmd.Flags().StringVar(&cu.destinationURL, "destination-url", "", "URL for HTTP destinations")
	cu.cmd.Flags().StringVar(&cu.destinationCliPath, "destination-cli-path", "/", "CLI path for CLI destinations (default: /)")

	// Destination authentication flags
	cu.cmd.Flags().StringVar(&cu.DestinationAuthMethod, "destination-auth-method", "", "Authentication method for HTTP destinations (e.g., bearer, basic, api_key, hmac, oauth2)")
	cu.cmd.Flags().StringVar(&cu.DestinationBearerToken, "destination-bearer-token", "", "Bearer token for destination authentication")
	cu.cmd.Flags().StringVar(&cu.DestinationBasicAuthUser, "destination-basic-auth-user", "", "Username for destination Basic authentication")
	cu.cmd.Flags().StringVar(&cu.DestinationBasicAuthPass, "destination-basic-auth-pass", "", "Password for destination Basic authentication")
	cu.cmd.Flags().StringVar(&cu.DestinationAPIKey, "destination-api-key", "", "API key for destination authentication")
	cu.cmd.Flags().StringVar(&cu.DestinationAPIKeyHeader, "destination-api-key-header", "Authorization", "Header to use for API key authentication")
	cu.cmd.Flags().StringVar(&cu.DestinationHMACSecret, "destination-hmac-secret", "", "HMAC secret for destination signature verification")
	cu.cmd.Flags().StringVar(&cu.DestinationHMACAlgo, "destination-hmac-algo", "sha256", "HMAC algorithm for destination signature")
	cu.cmd.Flags().StringVar(&cu.DestinationHMACHeader, "destination-hmac-header", "X-Signature-256", "Header to use for HMAC signature")
	cu.cmd.Flags().StringVar(&cu.DestinationOauthClientID, "destination-oauth-client-id", "", "OAuth2 client ID for destination authentication")
	cu.cmd.Flags().StringVar(&cu.DestinationOauthClientSecret, "destination-oauth-client-secret", "", "OAuth2 client secret for destination authentication")
	cu.cmd.Flags().StringVar(&cu.DestinationOauthTokenURL, "destination-oauth-token-url", "", "OAuth2 token URL for destination authentication")

	// Destination rate limiting flags
	cu.cmd.Flags().IntVar(&cu.DestinationRateLimit, "destination-rate-limit", 0, "Rate limit for destination (requests per period)")
	cu.cmd.Flags().StringVar(&cu.DestinationRateLimitPeriod, "destination-rate-limit-period", "", "Rate limit period (second, minute, hour)")

	// Rule flags - Retry
	cu.cmd.Flags().StringVar(&cu.RuleRetryStrategy, "rule-retry-strategy", "", "Retry strategy (linear, exponential)")
	cu.cmd.Flags().IntVar(&cu.RuleRetryCount, "rule-retry-count", 0, "Number of retry attempts")
	cu.cmd.Flags().IntVar(&cu.RuleRetryInterval, "rule-retry-interval", 0, "Interval between retries in milliseconds")
	cu.cmd.Flags().StringVar(&cu.RuleRetryResponseStatusCode, "rule-retry-response-status-codes", "", "Comma-separated HTTP status codes to retry on (e.g., '429,500,502')")

	// Rule flags - Filter
	cu.cmd.Flags().StringVar(&cu.RuleFilterBody, "rule-filter-body", "", "JQ expression to filter on request body")
	cu.cmd.Flags().StringVar(&cu.RuleFilterHeaders, "rule-filter-headers", "", "JQ expression to filter on request headers")
	cu.cmd.Flags().StringVar(&cu.RuleFilterQuery, "rule-filter-query", "", "JQ expression to filter on request query parameters")
	cu.cmd.Flags().StringVar(&cu.RuleFilterPath, "rule-filter-path", "", "JQ expression to filter on request path")

	// Rule flags - Transform
	cu.cmd.Flags().StringVar(&cu.RuleTransformName, "rule-transform-name", "", "Name or ID of the transformation to apply")
	cu.cmd.Flags().StringVar(&cu.RuleTransformCode, "rule-transform-code", "", "Transformation code (if creating inline)")
	cu.cmd.Flags().StringVar(&cu.RuleTransformEnv, "rule-transform-env", "", "JSON string representing environment variables for transformation")

	// Rule flags - Delay
	cu.cmd.Flags().IntVar(&cu.RuleDelay, "rule-delay", 0, "Delay in milliseconds")

	// Rule flags - Deduplicate
	cu.cmd.Flags().IntVar(&cu.RuleDeduplicateWindow, "rule-deduplicate-window", 0, "Time window in seconds for deduplication")
	cu.cmd.Flags().StringVar(&cu.RuleDeduplicateIncludeFields, "rule-deduplicate-include-fields", "", "Comma-separated list of fields to include for deduplication")
	cu.cmd.Flags().StringVar(&cu.RuleDeduplicateExcludeFields, "rule-deduplicate-exclude-fields", "", "Comma-separated list of fields to exclude for deduplication")

	// Rules JSON fallback
	cu.cmd.Flags().StringVar(&cu.Rules, "rules", "", "JSON string representing the entire rules array")
	cu.cmd.Flags().StringVar(&cu.RulesFile, "rules-file", "", "Path to a JSON file containing the rules array")

	// Reference existing resources
	cu.cmd.Flags().StringVar(&cu.sourceID, "source-id", "", "Use existing source by ID")
	cu.cmd.Flags().StringVar(&cu.destinationID, "destination-id", "", "Use existing destination by ID")

	// Output flags
	cu.cmd.Flags().StringVar(&cu.output, "output", "", "Output format (json)")

	// Upsert-specific flags
	cu.cmd.Flags().BoolVar(&cu.dryRun, "dry-run", false, "Preview changes without applying them")

	return cu
}

func (cu *connectionUpsertCmd) validateUpsertFlags(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	// Get name from positional argument
	name := args[0]
	cu.name = name

	// For dry-run, we don't need strict validation upfront
	if cu.dryRun {
		return nil
	}

	// Check if connection exists to determine validation mode
	client := Config.GetAPIClient()
	connections, err := client.ListConnections(context.Background(), map[string]string{
		"name": name,
	})
	if err != nil {
		return fmt.Errorf("failed to check if connection exists: %w", err)
	}

	// Connection exists - allow partial updates (relaxed validation)
	if connections != nil && len(connections.Models) > 0 {
		// For updates, only validate what's provided
		if cu.validateRules() != nil {
			return cu.validateRules()
		}
		if cu.validateRateLimiting() != nil {
			return cu.validateRateLimiting()
		}
		return nil
	}

	// Connection doesn't exist - require source and destination (same as create)
	return cu.connectionCreateCmd.validateFlags(cmd, args)
}

func (cu *connectionUpsertCmd) runConnectionUpsertCmd(cmd *cobra.Command, args []string) error {
	// Get name from positional argument
	name := args[0]
	cu.name = name

	client := Config.GetAPIClient()

	// Check if connection exists
	connections, err := client.ListConnections(context.Background(), map[string]string{
		"name": name,
	})
	if err != nil {
		return fmt.Errorf("failed to check if connection exists: %w", err)
	}

	var existing *hookdeck.Connection
	isUpdate := false
	if connections != nil && len(connections.Models) > 0 {
		existing = &connections.Models[0]
		isUpdate = true
	}

	// Build the request
	req := &hookdeck.ConnectionCreateRequest{
		Name: &cu.name,
	}
	if cu.description != "" {
		req.Description = &cu.description
	}

	// Handle Source
	if cu.sourceID != "" {
		req.SourceID = &cu.sourceID
	} else if cu.sourceName != "" || cu.sourceType != "" {
		sourceInput, err := cu.buildSourceInput()
		if err != nil {
			return err
		}
		req.Source = sourceInput
	} else if isUpdate && existing != nil && existing.Source != nil {
		// Preserve existing source when updating and no source flags provided
		req.SourceID = &existing.Source.ID
	}

	// Handle Destination
	if cu.destinationID != "" {
		req.DestinationID = &cu.destinationID
	} else if cu.destinationName != "" || cu.destinationType != "" {
		destinationInput, err := cu.buildDestinationInput()
		if err != nil {
			return err
		}
		req.Destination = destinationInput
	} else if isUpdate && existing != nil && existing.Destination != nil {
		// Preserve existing destination when updating and no destination flags provided
		req.DestinationID = &existing.Destination.ID
	}

	// Handle Rules
	rules, err := cu.buildRulesArray(cmd)
	if err != nil {
		return err
	}
	if len(rules) > 0 {
		req.Rules = rules
	}

	// Dry-run mode: preview changes without applying
	if cu.dryRun {
		return cu.previewUpsertChanges(existing, req, isUpdate)
	}

	// Execute the upsert
	if cu.output != "json" {
		if isUpdate {
			fmt.Printf("Updating connection '%s'...\n", cu.name)
		} else {
			fmt.Printf("Creating connection '%s'...\n", cu.name)
		}
	}

	connection, err := client.UpsertConnection(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to upsert connection: %w", err)
	}

	// Display results
	if cu.output == "json" {
		jsonBytes, err := json.MarshalIndent(connection, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal connection to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
	} else {
		if isUpdate {
			fmt.Printf("✓ Updated connection: %s\n", cu.name)
		} else {
			fmt.Printf("✓ Created connection: %s\n", cu.name)
		}

		fmt.Printf("\nConnection Details:\n")
		fmt.Printf("  ID: %s\n", connection.ID)
		if connection.Name != nil {
			fmt.Printf("  Name: %s\n", *connection.Name)
		}

		if connection.Source != nil {
			fmt.Printf("  Source: %s (%s)\n", connection.Source.Name, connection.Source.ID)
		}

		if connection.Destination != nil {
			fmt.Printf("  Destination: %s (%s)\n", connection.Destination.Name, connection.Destination.ID)
		}

		if len(connection.Rules) > 0 {
			fmt.Printf("  Rules: %d configured\n", len(connection.Rules))
		}
	}

	return nil
}

func (cu *connectionUpsertCmd) previewUpsertChanges(existing *hookdeck.Connection, req *hookdeck.ConnectionCreateRequest, isUpdate bool) error {
	fmt.Printf("=== DRY RUN MODE ===\n\n")

	if isUpdate {
		fmt.Printf("Operation: UPDATE\n")
		fmt.Printf("Connection: %s (ID: %s)\n\n", cu.name, existing.ID)

		fmt.Printf("Changes to be applied:\n")
		changes := 0

		// Check description changes
		if req.Description != nil {
			changes++
			currentDesc := ""
			if existing.Description != nil {
				currentDesc = *existing.Description
			}
			fmt.Printf("  • Description: \"%s\" → \"%s\"\n", currentDesc, *req.Description)
		}

		// Check source changes
		if req.SourceID != nil || req.Source != nil {
			changes++
			fmt.Printf("  • Source: ")
			if req.SourceID != nil {
				fmt.Printf("%s → %s (by ID)\n", existing.Source.ID, *req.SourceID)
			} else if req.Source != nil {
				fmt.Printf("%s → %s (inline creation)\n", existing.Source.Name, req.Source.Name)
			}
		}

		// Check destination changes
		if req.DestinationID != nil || req.Destination != nil {
			changes++
			fmt.Printf("  • Destination: ")
			if req.DestinationID != nil {
				fmt.Printf("%s → %s (by ID)\n", existing.Destination.ID, *req.DestinationID)
			} else if req.Destination != nil {
				fmt.Printf("%s → %s (inline creation)\n", existing.Destination.Name, req.Destination.Name)
			}
		}

		// Check rules changes
		if len(req.Rules) > 0 {
			changes++
			rulesJSON, _ := json.MarshalIndent(req.Rules, "    ", "  ")
			fmt.Printf("  • Rules:\n")
			fmt.Printf("    Current: %d rules\n", len(existing.Rules))
			fmt.Printf("    New: %s\n", string(rulesJSON))
		}

		if changes == 0 {
			fmt.Printf("  No changes detected - connection will remain unchanged\n")
		}

		fmt.Printf("\nProperties preserved (not specified in command):\n")
		if req.SourceID == nil && req.Source == nil && existing.Source != nil {
			fmt.Printf("  • Source: %s (unchanged)\n", existing.Source.Name)
		}
		if req.DestinationID == nil && req.Destination == nil && existing.Destination != nil {
			fmt.Printf("  • Destination: %s (unchanged)\n", existing.Destination.Name)
		}
		if len(req.Rules) == 0 && len(existing.Rules) > 0 {
			fmt.Printf("  • Rules: %d rules (unchanged)\n", len(existing.Rules))
		}
	} else {
		fmt.Printf("Operation: CREATE\n")
		fmt.Printf("Connection: %s\n\n", cu.name)

		fmt.Printf("Configuration to be created:\n")

		if req.Description != nil {
			fmt.Printf("  • Description: %s\n", *req.Description)
		}

		if req.SourceID != nil {
			fmt.Printf("  • Source: %s (existing, by ID)\n", *req.SourceID)
		} else if req.Source != nil {
			fmt.Printf("  • Source: %s (type: %s, inline creation)\n", req.Source.Name, req.Source.Type)
		}

		if req.DestinationID != nil {
			fmt.Printf("  • Destination: %s (existing, by ID)\n", *req.DestinationID)
		} else if req.Destination != nil {
			fmt.Printf("  • Destination: %s (type: %s, inline creation)\n", req.Destination.Name, req.Destination.Type)
		}

		if len(req.Rules) > 0 {
			rulesJSON, _ := json.MarshalIndent(req.Rules, "    ", "  ")
			fmt.Printf("  • Rules: %s\n", string(rulesJSON))
		}
	}

	fmt.Printf("\n=== DRY RUN COMPLETE ===\n")
	fmt.Printf("No changes were made. Remove --dry-run to apply these changes.\n")

	return nil
}

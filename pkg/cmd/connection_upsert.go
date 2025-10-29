package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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

	// Use a string flag to allow explicit true/false values
	var pathForwardingDisabledStr string
	cu.cmd.Flags().StringVar(&pathForwardingDisabledStr, "destination-path-forwarding-disabled", "", "Disable path forwarding for HTTP destinations (true/false)")

	// Parse the string value in PreRunE (will be handled by the existing PreRunE chain)
	originalPreRunE := cu.cmd.PreRunE
	cu.cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if pathForwardingDisabledStr != "" {
			val := pathForwardingDisabledStr == "true"
			cu.destinationPathForwardingDisabled = &val
		}
		if originalPreRunE != nil {
			return originalPreRunE(cmd, args)
		}
		return nil
	}

	cu.cmd.Flags().StringVar(&cu.destinationHTTPMethod, "destination-http-method", "", "HTTP method for HTTP destinations (GET, POST, PUT, PATCH, DELETE)")

	// Destination authentication flags
	cu.cmd.Flags().StringVar(&cu.DestinationAuthMethod, "destination-auth-method", "", "Authentication method for HTTP destinations (hookdeck, bearer, basic, api_key, custom_signature, oauth2_client_credentials, oauth2_authorization_code, aws)")

	// Bearer Token
	cu.cmd.Flags().StringVar(&cu.DestinationBearerToken, "destination-bearer-token", "", "Bearer token for destination authentication")

	// Basic Auth
	cu.cmd.Flags().StringVar(&cu.DestinationBasicAuthUser, "destination-basic-auth-user", "", "Username for destination Basic authentication")
	cu.cmd.Flags().StringVar(&cu.DestinationBasicAuthPass, "destination-basic-auth-pass", "", "Password for destination Basic authentication")

	// API Key
	cu.cmd.Flags().StringVar(&cu.DestinationAPIKey, "destination-api-key", "", "API key for destination authentication")
	cu.cmd.Flags().StringVar(&cu.DestinationAPIKeyHeader, "destination-api-key-header", "", "Key/header name for API key authentication")
	cu.cmd.Flags().StringVar(&cu.DestinationAPIKeyTo, "destination-api-key-to", "header", "Where to send API key: 'header' or 'query'")

	// Custom Signature (HMAC)
	cu.cmd.Flags().StringVar(&cu.DestinationCustomSignatureKey, "destination-custom-signature-key", "", "Key/header name for custom signature")
	cu.cmd.Flags().StringVar(&cu.DestinationCustomSignatureSecret, "destination-custom-signature-secret", "", "Signing secret for custom signature")

	// OAuth2 (shared flags for both Client Credentials and Authorization Code)
	cu.cmd.Flags().StringVar(&cu.DestinationOAuth2AuthServer, "destination-oauth2-auth-server", "", "OAuth2 authorization server URL")
	cu.cmd.Flags().StringVar(&cu.DestinationOAuth2ClientID, "destination-oauth2-client-id", "", "OAuth2 client ID")
	cu.cmd.Flags().StringVar(&cu.DestinationOAuth2ClientSecret, "destination-oauth2-client-secret", "", "OAuth2 client secret")
	cu.cmd.Flags().StringVar(&cu.DestinationOAuth2Scopes, "destination-oauth2-scopes", "", "OAuth2 scopes (comma-separated)")
	cu.cmd.Flags().StringVar(&cu.DestinationOAuth2AuthType, "destination-oauth2-auth-type", "basic", "OAuth2 Client Credentials authentication type: 'basic', 'bearer', or 'x-www-form-urlencoded'")

	// OAuth2 Authorization Code specific
	cu.cmd.Flags().StringVar(&cu.DestinationOAuth2RefreshToken, "destination-oauth2-refresh-token", "", "OAuth2 refresh token (required for Authorization Code flow)")

	// AWS Signature
	cu.cmd.Flags().StringVar(&cu.DestinationAWSAccessKeyID, "destination-aws-access-key-id", "", "AWS access key ID")
	cu.cmd.Flags().StringVar(&cu.DestinationAWSSecretAccessKey, "destination-aws-secret-access-key", "", "AWS secret access key")
	cu.cmd.Flags().StringVar(&cu.DestinationAWSRegion, "destination-aws-region", "", "AWS region")
	cu.cmd.Flags().StringVar(&cu.DestinationAWSService, "destination-aws-service", "", "AWS service name")

	// Destination rate limiting flags
	cu.cmd.Flags().IntVar(&cu.DestinationRateLimit, "destination-rate-limit", 0, "Rate limit for destination (requests per period)")
	cu.cmd.Flags().StringVar(&cu.DestinationRateLimitPeriod, "destination-rate-limit-period", "", "Rate limit period (second, minute, hour, concurrent)")

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

	// For dry-run, we allow any combination of flags (will check existence during execution)
	if cu.dryRun {
		return nil
	}

	// For normal upsert, validate internal flag consistency only
	// We don't check if connection exists - let the API handle validation

	// Validate rules if provided
	if cu.hasAnyRuleFlag() {
		if err := cu.validateRules(); err != nil {
			return err
		}
	}

	// Validate rate limiting if provided
	if cu.hasAnyRateLimitFlag() {
		if err := cu.validateRateLimiting(); err != nil {
			return err
		}
	}

	// If source or destination flags are provided, validate them
	if cu.hasAnySourceFlag() {
		if err := cu.validateSourceFlags(); err != nil {
			return err
		}
	}

	if cu.hasAnyDestinationFlag() {
		if err := cu.validateDestinationFlags(); err != nil {
			return err
		}
	}

	return nil
}

// Helper to check if any source flags are set
func (cu *connectionUpsertCmd) hasAnySourceFlag() bool {
	return cu.sourceName != "" || cu.sourceType != "" || cu.sourceID != ""
}

// Helper to check if any destination flags are set
func (cu *connectionUpsertCmd) hasAnyDestinationFlag() bool {
	return cu.destinationName != "" || cu.destinationType != "" || cu.destinationID != "" || cu.destinationURL != "" ||
		cu.destinationPathForwardingDisabled != nil || cu.destinationHTTPMethod != "" ||
		cu.DestinationRateLimit != 0 || cu.DestinationRateLimitPeriod != "" ||
		cu.DestinationAuthMethod != ""
}

// Helper to check if any rule flags are set
func (cu *connectionUpsertCmd) hasAnyRuleFlag() bool {
	return cu.RuleRetryStrategy != "" || cu.RuleFilterBody != "" || cu.RuleTransformName != "" ||
		cu.RuleDelay != 0 || cu.RuleDeduplicateWindow != 0 || cu.Rules != "" || cu.RulesFile != ""
}

// Helper to check if any rate limit flags are set
func (cu *connectionUpsertCmd) hasAnyRateLimitFlag() bool {
	return cu.DestinationRateLimit != 0 || cu.DestinationRateLimitPeriod != ""
}

// Validate source flags for consistency
func (cu *connectionUpsertCmd) validateSourceFlags() error {
	// If using source-id, don't allow inline creation flags
	if cu.sourceID != "" && (cu.sourceName != "" || cu.sourceType != "") {
		return fmt.Errorf("cannot use --source-id with --source-name or --source-type")
	}

	// If creating inline, require both name and type
	if (cu.sourceName != "" || cu.sourceType != "") && (cu.sourceName == "" || cu.sourceType == "") {
		return fmt.Errorf("both --source-name and --source-type are required for inline source creation")
	}

	return nil
}

// Validate destination flags for consistency
func (cu *connectionUpsertCmd) validateDestinationFlags() error {
	// If using destination-id, don't allow inline creation flags
	if cu.destinationID != "" && (cu.destinationName != "" || cu.destinationType != "") {
		return fmt.Errorf("cannot use --destination-id with --destination-name or --destination-type")
	}

	// If creating inline, require both name and type
	if (cu.destinationName != "" || cu.destinationType != "") && (cu.destinationName == "" || cu.destinationType == "") {
		return fmt.Errorf("both --destination-name and --destination-type are required for inline destination creation")
	}

	return nil
}

func (cu *connectionUpsertCmd) runConnectionUpsertCmd(cmd *cobra.Command, args []string) error {
	// Get name from positional argument
	name := args[0]
	cu.name = name

	client := Config.GetAPIClient()

	// Determine if we need to fetch existing connection
	// Only needed when:
	// 1. Dry-run mode (to show preview)
	// 2. Partial update (source/destination config fields without name/type)
	// 3. Updating config fields without recreating the resource
	hasDestinationConfigOnly := (cu.destinationPathForwardingDisabled != nil || cu.destinationHTTPMethod != "" ||
		cu.DestinationRateLimit != 0 || cu.DestinationAuthMethod != "") &&
		cu.destinationName == "" && cu.destinationType == "" && cu.destinationID == ""

	needsExisting := cu.dryRun || (!cu.hasAnySourceFlag() && !cu.hasAnyDestinationFlag()) || hasDestinationConfigOnly

	var existing *hookdeck.Connection
	var isUpdate bool

	if needsExisting {
		connections, err := client.ListConnections(context.Background(), map[string]string{
			"name": name,
		})
		if err != nil {
			return fmt.Errorf("failed to check if connection exists: %w", err)
		}

		if connections != nil && len(connections.Models) > 0 {
			existing = &connections.Models[0]
			isUpdate = true
		}
	}

	// Build the request
	req, err := cu.buildUpsertRequest(existing, isUpdate)
	if err != nil {
		return err
	}

	// For dry-run mode, preview changes without applying
	if cu.dryRun {
		return cu.previewUpsertChanges(existing, req, isUpdate)
	}

	// Execute the upsert
	if cu.output != "json" {
		fmt.Printf("Upserting connection '%s'...\n", cu.name)
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
		fmt.Printf("✓ Connection upserted: %s\n", cu.name)

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

// buildUpsertRequest constructs the upsert request from flags
// existing and isUpdate are used to preserve unspecified fields when doing partial updates
func (cu *connectionUpsertCmd) buildUpsertRequest(existing *hookdeck.Connection, isUpdate bool) (*hookdeck.ConnectionCreateRequest, error) {
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
			return nil, err
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
			return nil, err
		}
		req.Destination = destinationInput
	} else if isUpdate && existing != nil && existing.Destination != nil {
		// Check if any destination config fields are being updated
		hasDestinationConfigUpdate := cu.destinationPathForwardingDisabled != nil ||
			cu.destinationHTTPMethod != "" ||
			cu.DestinationRateLimit != 0 || cu.DestinationRateLimitPeriod != "" ||
			cu.DestinationAuthMethod != ""

		if hasDestinationConfigUpdate {
			// For partial config updates, we need to send the full destination object
			// with the updated config merged in
			destinationInput, err := cu.buildDestinationInputForUpdate(existing.Destination)
			if err != nil {
				return nil, err
			}
			req.Destination = destinationInput
		} else {
			// Preserve existing destination when updating and no destination flags provided
			req.DestinationID = &existing.Destination.ID
		}
	}

	// Also preserve source if not specified
	if req.SourceID == nil && req.Source == nil && isUpdate && existing != nil && existing.Source != nil {
		req.SourceID = &existing.Source.ID
	}

	// Handle Rules
	rules, err := cu.buildRulesArray(nil)
	if err != nil {
		return nil, err
	}
	if len(rules) > 0 {
		req.Rules = rules
	}

	return req, nil
}

// buildDestinationInputForUpdate builds a destination input for partial config updates
// It merges the existing destination config with any new flags provided
func (cu *connectionUpsertCmd) buildDestinationInputForUpdate(existingDest *hookdeck.Destination) (*hookdeck.DestinationCreateInput, error) {
	// Start with the existing destination
	input := &hookdeck.DestinationCreateInput{
		Name:        existingDest.Name,
		Type:        existingDest.Type,
		Description: existingDest.Description,
	}

	// Get existing config or create new one
	destConfig := make(map[string]interface{})
	if existingDest.Config != nil {
		// Copy existing config
		for k, v := range existingDest.Config {
			destConfig[k] = v
		}
	}

	// Apply any new config values from flags
	if cu.destinationURL != "" {
		destConfig["url"] = cu.destinationURL
	}

	if cu.destinationPathForwardingDisabled != nil {
		destConfig["path_forwarding_disabled"] = *cu.destinationPathForwardingDisabled
	}

	if cu.destinationHTTPMethod != "" {
		// Validate HTTP method
		validMethods := map[string]bool{
			"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true,
		}
		method := strings.ToUpper(cu.destinationHTTPMethod)
		if !validMethods[method] {
			return nil, fmt.Errorf("--destination-http-method must be one of: GET, POST, PUT, PATCH, DELETE")
		}
		destConfig["http_method"] = method
	}

	// Apply rate limiting if provided
	if cu.DestinationRateLimit > 0 {
		destConfig["rate_limit"] = cu.DestinationRateLimit
		destConfig["rate_limit_period"] = cu.DestinationRateLimitPeriod
	}

	// Apply authentication config if provided
	if cu.DestinationAuthMethod != "" {
		authConfig, err := cu.buildAuthConfig()
		if err != nil {
			return nil, err
		}
		if len(authConfig) > 0 {
			destConfig["auth_method"] = authConfig
		}
	}

	input.Config = destConfig
	return input, nil
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

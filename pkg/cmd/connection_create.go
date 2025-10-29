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

	// Source configuration flags
	SourceAllowedHTTPMethods string
	SourceCustomResponseType string
	SourceCustomResponseBody string

	// JSON config fallback
	SourceConfig     string
	SourceConfigFile string

	// Destination flags (inline creation)
	destinationName                   string
	destinationType                   string
	destinationDescription            string
	destinationURL                    string
	destinationCliPath                string
	destinationPathForwardingDisabled *bool
	destinationHTTPMethod             string

	// Destination authentication flags
	DestinationAuthMethod    string
	DestinationBearerToken   string
	DestinationBasicAuthUser string
	DestinationBasicAuthPass string
	DestinationAPIKey        string
	DestinationAPIKeyHeader  string
	DestinationAPIKeyTo      string // "header" or "query"

	// Custom Signature (HMAC) flags
	DestinationCustomSignatureKey    string
	DestinationCustomSignatureSecret string

	// OAuth2 flags (shared between Client Credentials and Authorization Code)
	DestinationOAuth2AuthServer   string
	DestinationOAuth2ClientID     string
	DestinationOAuth2ClientSecret string
	DestinationOAuth2Scopes       string
	DestinationOAuth2AuthType     string // "basic", "bearer", or "x-www-form-urlencoded" (Client Credentials only)

	// OAuth2 Authorization Code specific flags
	DestinationOAuth2RefreshToken string

	// AWS Signature flags
	DestinationAWSAccessKeyID     string
	DestinationAWSSecretAccessKey string
	DestinationAWSRegion          string
	DestinationAWSService         string

	// Destination rate limiting flags
	DestinationRateLimit       int
	DestinationRateLimitPeriod string

	// Rule flags - Retry
	RuleRetryStrategy           string
	RuleRetryCount              int
	RuleRetryInterval           int
	RuleRetryResponseStatusCode string

	// Rule flags - Filter
	RuleFilterBody    string
	RuleFilterHeaders string
	RuleFilterQuery   string
	RuleFilterPath    string

	// Rule flags - Transform
	RuleTransformName string
	RuleTransformCode string
	RuleTransformEnv  string

	// Rule flags - Delay
	RuleDelay int

	// Rule flags - Deduplicate
	RuleDeduplicateWindow        int
	RuleDeduplicateIncludeFields string
	RuleDeduplicateExcludeFields string

	// Rules JSON fallback
	Rules     string
	RulesFile string

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
		   --destination-id dst_def456
	
		 # Create with source configuration options
		 hookdeck connection create \
		   --name "api-webhooks" \
		   --source-type WEBHOOK --source-name "api-source" \
		   --source-allowed-http-methods "POST,PUT,PATCH" \
		   --source-custom-response-content-type "json" \
		   --source-custom-response-body '{"status":"received"}' \
		   --destination-type CLI --destination-name "local-dev"`,
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

	// Source configuration flags
	cc.cmd.Flags().StringVar(&cc.SourceAllowedHTTPMethods, "source-allowed-http-methods", "", "Comma-separated list of allowed HTTP methods (GET, POST, PUT, PATCH, DELETE)")
	cc.cmd.Flags().StringVar(&cc.SourceCustomResponseType, "source-custom-response-content-type", "", "Custom response content type (json, text, xml)")
	cc.cmd.Flags().StringVar(&cc.SourceCustomResponseBody, "source-custom-response-body", "", "Custom response body (max 1000 chars)")

	// JSON config fallback
	cc.cmd.Flags().StringVar(&cc.SourceConfig, "source-config", "", "JSON string for source authentication config")
	cc.cmd.Flags().StringVar(&cc.SourceConfigFile, "source-config-file", "", "Path to a JSON file for source authentication config")

	// Destination inline creation flags
	cc.cmd.Flags().StringVar(&cc.destinationName, "destination-name", "", "Destination name for inline creation")
	cc.cmd.Flags().StringVar(&cc.destinationType, "destination-type", "", "Destination type (CLI, HTTP, MOCK)")
	cc.cmd.Flags().StringVar(&cc.destinationDescription, "destination-description", "", "Destination description")
	cc.cmd.Flags().StringVar(&cc.destinationURL, "destination-url", "", "URL for HTTP destinations")
	cc.cmd.Flags().StringVar(&cc.destinationCliPath, "destination-cli-path", "/", "CLI path for CLI destinations (default: /)")

	// Use a string flag to allow explicit true/false values
	var pathForwardingDisabledStr string
	cc.cmd.Flags().StringVar(&pathForwardingDisabledStr, "destination-path-forwarding-disabled", "", "Disable path forwarding for HTTP destinations (true/false)")

	// Parse the string value in PreRunE
	cc.cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if pathForwardingDisabledStr != "" {
			val := pathForwardingDisabledStr == "true"
			cc.destinationPathForwardingDisabled = &val
		}
		return cc.validateFlags(cmd, args)
	}

	cc.cmd.Flags().StringVar(&cc.destinationHTTPMethod, "destination-http-method", "", "HTTP method for HTTP destinations (GET, POST, PUT, PATCH, DELETE)")

	// Destination authentication flags
	cc.cmd.Flags().StringVar(&cc.DestinationAuthMethod, "destination-auth-method", "", "Authentication method for HTTP destinations (hookdeck, bearer, basic, api_key, custom_signature, oauth2_client_credentials, oauth2_authorization_code, aws)")

	// Bearer Token
	cc.cmd.Flags().StringVar(&cc.DestinationBearerToken, "destination-bearer-token", "", "Bearer token for destination authentication")

	// Basic Auth
	cc.cmd.Flags().StringVar(&cc.DestinationBasicAuthUser, "destination-basic-auth-user", "", "Username for destination Basic authentication")
	cc.cmd.Flags().StringVar(&cc.DestinationBasicAuthPass, "destination-basic-auth-pass", "", "Password for destination Basic authentication")

	// API Key
	cc.cmd.Flags().StringVar(&cc.DestinationAPIKey, "destination-api-key", "", "API key for destination authentication")
	cc.cmd.Flags().StringVar(&cc.DestinationAPIKeyHeader, "destination-api-key-header", "", "Key/header name for API key authentication")
	cc.cmd.Flags().StringVar(&cc.DestinationAPIKeyTo, "destination-api-key-to", "header", "Where to send API key: 'header' or 'query'")

	// Custom Signature (HMAC)
	cc.cmd.Flags().StringVar(&cc.DestinationCustomSignatureKey, "destination-custom-signature-key", "", "Key/header name for custom signature")
	cc.cmd.Flags().StringVar(&cc.DestinationCustomSignatureSecret, "destination-custom-signature-secret", "", "Signing secret for custom signature")

	// OAuth2 (shared flags for both Client Credentials and Authorization Code)
	cc.cmd.Flags().StringVar(&cc.DestinationOAuth2AuthServer, "destination-oauth2-auth-server", "", "OAuth2 authorization server URL")
	cc.cmd.Flags().StringVar(&cc.DestinationOAuth2ClientID, "destination-oauth2-client-id", "", "OAuth2 client ID")
	cc.cmd.Flags().StringVar(&cc.DestinationOAuth2ClientSecret, "destination-oauth2-client-secret", "", "OAuth2 client secret")
	cc.cmd.Flags().StringVar(&cc.DestinationOAuth2Scopes, "destination-oauth2-scopes", "", "OAuth2 scopes (comma-separated)")
	cc.cmd.Flags().StringVar(&cc.DestinationOAuth2AuthType, "destination-oauth2-auth-type", "basic", "OAuth2 Client Credentials authentication type: 'basic', 'bearer', or 'x-www-form-urlencoded'")

	// OAuth2 Authorization Code specific
	cc.cmd.Flags().StringVar(&cc.DestinationOAuth2RefreshToken, "destination-oauth2-refresh-token", "", "OAuth2 refresh token (required for Authorization Code flow)")

	// AWS Signature
	cc.cmd.Flags().StringVar(&cc.DestinationAWSAccessKeyID, "destination-aws-access-key-id", "", "AWS access key ID")
	cc.cmd.Flags().StringVar(&cc.DestinationAWSSecretAccessKey, "destination-aws-secret-access-key", "", "AWS secret access key")
	cc.cmd.Flags().StringVar(&cc.DestinationAWSRegion, "destination-aws-region", "", "AWS region")
	cc.cmd.Flags().StringVar(&cc.DestinationAWSService, "destination-aws-service", "", "AWS service name")

	// Destination rate limiting flags
	cc.cmd.Flags().IntVar(&cc.DestinationRateLimit, "destination-rate-limit", 0, "Rate limit for destination (requests per period)")
	cc.cmd.Flags().StringVar(&cc.DestinationRateLimitPeriod, "destination-rate-limit-period", "", "Rate limit period (second, minute, hour, concurrent)")

	// Rule flags - Retry
	cc.cmd.Flags().StringVar(&cc.RuleRetryStrategy, "rule-retry-strategy", "", "Retry strategy (linear, exponential)")
	cc.cmd.Flags().IntVar(&cc.RuleRetryCount, "rule-retry-count", 0, "Number of retry attempts")
	cc.cmd.Flags().IntVar(&cc.RuleRetryInterval, "rule-retry-interval", 0, "Interval between retries in milliseconds")
	cc.cmd.Flags().StringVar(&cc.RuleRetryResponseStatusCode, "rule-retry-response-status-codes", "", "Comma-separated HTTP status codes to retry on (e.g., '429,500,502')")

	// Rule flags - Filter
	cc.cmd.Flags().StringVar(&cc.RuleFilterBody, "rule-filter-body", "", "JQ expression to filter on request body")
	cc.cmd.Flags().StringVar(&cc.RuleFilterHeaders, "rule-filter-headers", "", "JQ expression to filter on request headers")
	cc.cmd.Flags().StringVar(&cc.RuleFilterQuery, "rule-filter-query", "", "JQ expression to filter on request query parameters")
	cc.cmd.Flags().StringVar(&cc.RuleFilterPath, "rule-filter-path", "", "JQ expression to filter on request path")

	// Rule flags - Transform
	cc.cmd.Flags().StringVar(&cc.RuleTransformName, "rule-transform-name", "", "Name or ID of the transformation to apply")
	cc.cmd.Flags().StringVar(&cc.RuleTransformCode, "rule-transform-code", "", "Transformation code (if creating inline)")
	cc.cmd.Flags().StringVar(&cc.RuleTransformEnv, "rule-transform-env", "", "JSON string representing environment variables for transformation")

	// Rule flags - Delay
	cc.cmd.Flags().IntVar(&cc.RuleDelay, "rule-delay", 0, "Delay in milliseconds")

	// Rule flags - Deduplicate
	cc.cmd.Flags().IntVar(&cc.RuleDeduplicateWindow, "rule-deduplicate-window", 0, "Time window in seconds for deduplication")
	cc.cmd.Flags().StringVar(&cc.RuleDeduplicateIncludeFields, "rule-deduplicate-include-fields", "", "Comma-separated list of fields to include for deduplication")
	cc.cmd.Flags().StringVar(&cc.RuleDeduplicateExcludeFields, "rule-deduplicate-exclude-fields", "", "Comma-separated list of fields to exclude for deduplication")

	// Rules JSON fallback
	cc.cmd.Flags().StringVar(&cc.Rules, "rules", "", "JSON string representing the entire rules array")
	cc.cmd.Flags().StringVar(&cc.RulesFile, "rules-file", "", "Path to a JSON file containing the rules array")

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

	// Validate rules configuration
	if err := cc.validateRules(); err != nil {
		return err
	}

	// Validate rate limiting configuration
	if err := cc.validateRateLimiting(); err != nil {
		return err
	}

	return nil
}

func (cc *connectionCreateCmd) validateRules() error {
	// Check if JSON fallback is used
	hasJSONRules := cc.Rules != "" || cc.RulesFile != ""

	// Check if any individual rule flags are set
	hasRetryFlags := cc.RuleRetryStrategy != "" || cc.RuleRetryCount > 0 || cc.RuleRetryInterval > 0 || cc.RuleRetryResponseStatusCode != ""
	hasFilterFlags := cc.RuleFilterBody != "" || cc.RuleFilterHeaders != "" || cc.RuleFilterQuery != "" || cc.RuleFilterPath != ""
	hasTransformFlags := cc.RuleTransformName != "" || cc.RuleTransformCode != "" || cc.RuleTransformEnv != ""
	hasDelayFlags := cc.RuleDelay > 0
	hasDeduplicateFlags := cc.RuleDeduplicateWindow > 0 || cc.RuleDeduplicateIncludeFields != "" || cc.RuleDeduplicateExcludeFields != ""

	hasIndividualFlags := hasRetryFlags || hasFilterFlags || hasTransformFlags || hasDelayFlags || hasDeduplicateFlags

	// If JSON fallback is used, individual flags must not be set
	if hasJSONRules && hasIndividualFlags {
		return fmt.Errorf("cannot use --rules or --rules-file with individual --rule-* flags")
	}

	// Validate retry rule
	if hasRetryFlags {
		if cc.RuleRetryStrategy == "" {
			return fmt.Errorf("--rule-retry-strategy is required when using retry rule flags")
		}
		if cc.RuleRetryStrategy != "linear" && cc.RuleRetryStrategy != "exponential" {
			return fmt.Errorf("--rule-retry-strategy must be 'linear' or 'exponential', got: %s", cc.RuleRetryStrategy)
		}
		if cc.RuleRetryCount < 0 {
			return fmt.Errorf("--rule-retry-count must be a positive integer")
		}
		if cc.RuleRetryInterval < 0 {
			return fmt.Errorf("--rule-retry-interval must be a positive integer")
		}
	}

	// Validate filter rule
	if hasFilterFlags {
		if cc.RuleFilterBody == "" && cc.RuleFilterHeaders == "" && cc.RuleFilterQuery == "" && cc.RuleFilterPath == "" {
			return fmt.Errorf("at least one filter expression must be provided when using filter rule flags")
		}
	}

	// Validate transform rule
	if hasTransformFlags {
		if cc.RuleTransformName == "" {
			return fmt.Errorf("--rule-transform-name is required when using transform rule flags")
		}
		if cc.RuleTransformEnv != "" {
			// Validate JSON
			var env map[string]interface{}
			if err := json.Unmarshal([]byte(cc.RuleTransformEnv), &env); err != nil {
				return fmt.Errorf("--rule-transform-env must be a valid JSON string: %w", err)
			}
		}
	}

	// Validate delay rule
	if hasDelayFlags {
		if cc.RuleDelay < 0 {
			return fmt.Errorf("--rule-delay must be a positive integer")
		}
	}

	// Validate deduplicate rule
	if hasDeduplicateFlags {
		if cc.RuleDeduplicateWindow == 0 {
			return fmt.Errorf("--rule-deduplicate-window is required when using deduplicate rule flags")
		}
		if cc.RuleDeduplicateWindow < 0 {
			return fmt.Errorf("--rule-deduplicate-window must be a positive integer")
		}
	}

	return nil
}

func (cc *connectionCreateCmd) validateRateLimiting() error {
	hasRateLimit := cc.DestinationRateLimit > 0 || cc.DestinationRateLimitPeriod != ""

	if hasRateLimit {
		if cc.DestinationRateLimit <= 0 {
			return fmt.Errorf("--destination-rate-limit must be a positive integer when rate limiting is configured")
		}
		if cc.DestinationRateLimitPeriod == "" {
			return fmt.Errorf("--destination-rate-limit-period is required when --destination-rate-limit is set")
		}
		// Let API validate the period value (supports: second, minute, hour, concurrent)
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

	// Handle Rules
	rules, err := cc.buildRulesArray(cmd)
	if err != nil {
		return err
	}
	if len(rules) > 0 {
		req.Rules = rules
	}

	if cc.output != "json" {
		fmt.Printf("Creating connection '%s'...\n", cc.name)
	}

	// Single API call to create the connection
	connection, err := client.CreateConnection(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to create connection: %w", err)
	}

	// Verify the created connection by fetching it again
	if cc.output != "json" {
		fmt.Println("Verifying created connection...")
	}
	verifiedConnection, err := client.GetConnection(context.Background(), connection.ID)
	if err != nil {
		return fmt.Errorf("failed to verify connection: %w", err)
	}

	// Quick integrity check
	if verifiedConnection.Source != nil && cc.sourceType != "" && strings.ToUpper(cc.sourceType) != verifiedConnection.Source.Type {
		return fmt.Errorf("Source type mismatch for connection %s.\nExpected: %s, Got: %s",
			verifiedConnection.ID, strings.ToUpper(cc.sourceType), verifiedConnection.Source.Type)
	}

	// Display results
	if cc.output == "json" {
		jsonBytes, err := json.MarshalIndent(verifiedConnection, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal connection to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
	} else {
		fmt.Printf("Successfully created connection with ID: %s\n", verifiedConnection.ID)

		if verifiedConnection.Name != nil {
			fmt.Printf("Connection:  %s (%s)\n", *verifiedConnection.Name, verifiedConnection.ID)
		} else {
			fmt.Printf("Connection:  (unnamed)\n")
		}

		if verifiedConnection.Source != nil {
			fmt.Printf("Source:      %s (%s)\n", verifiedConnection.Source.Name, verifiedConnection.Source.ID)
			fmt.Printf("Source URL:  %s\n", verifiedConnection.Source.URL)
		}

		if verifiedConnection.Destination != nil {
			fmt.Printf("Destination: %s (%s)\n", verifiedConnection.Destination.Name, verifiedConnection.Destination.ID)
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

		// Add HTTP-specific optional fields
		if cc.destinationPathForwardingDisabled != nil {
			destinationConfig["path_forwarding_disabled"] = *cc.destinationPathForwardingDisabled
		}
		if cc.destinationHTTPMethod != "" {
			// Validate HTTP method
			validMethods := map[string]bool{
				"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true,
			}
			method := strings.ToUpper(cc.destinationHTTPMethod)
			if !validMethods[method] {
				return nil, fmt.Errorf("--destination-http-method must be one of: GET, POST, PUT, PATCH, DELETE")
			}
			destinationConfig["http_method"] = method
		}
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

	// Build authentication configuration
	authConfig, err := cc.buildAuthConfig()
	if err != nil {
		return nil, err
	}

	if len(authConfig) > 0 {
		config["auth_method"] = authConfig
	}

	// Add rate limiting configuration
	if cc.DestinationRateLimit > 0 {
		config["rate_limit"] = cc.DestinationRateLimit
		config["rate_limit_period"] = cc.DestinationRateLimitPeriod
	}

	if len(config) == 0 {
		return make(map[string]interface{}), nil
	}

	return config, nil
}

func (cc *connectionCreateCmd) buildAuthConfig() (map[string]interface{}, error) {
	authConfig := make(map[string]interface{})

	switch cc.DestinationAuthMethod {
	case "hookdeck", "":
		// HOOKDECK_SIGNATURE - default, no config needed
		// Empty string means default to Hookdeck signature
		if cc.DestinationAuthMethod == "hookdeck" {
			authConfig["type"] = "HOOKDECK_SIGNATURE"
		}
		// If empty, don't set auth at all (API will default to Hookdeck signature)

	case "bearer":
		// BEARER_TOKEN
		if cc.DestinationBearerToken == "" {
			return nil, fmt.Errorf("--destination-bearer-token is required for bearer auth method")
		}
		authConfig["type"] = "BEARER_TOKEN"
		authConfig["token"] = cc.DestinationBearerToken

	case "basic":
		// BASIC_AUTH
		if cc.DestinationBasicAuthUser == "" || cc.DestinationBasicAuthPass == "" {
			return nil, fmt.Errorf("--destination-basic-auth-user and --destination-basic-auth-pass are required for basic auth method")
		}
		authConfig["type"] = "BASIC_AUTH"
		authConfig["username"] = cc.DestinationBasicAuthUser
		authConfig["password"] = cc.DestinationBasicAuthPass

	case "api_key":
		// API_KEY
		if cc.DestinationAPIKey == "" {
			return nil, fmt.Errorf("--destination-api-key is required for api_key auth method")
		}
		authConfig["type"] = "API_KEY"
		authConfig["api_key"] = cc.DestinationAPIKey

		// Key/header name is required
		if cc.DestinationAPIKeyHeader == "" {
			return nil, fmt.Errorf("--destination-api-key-header is required for api_key auth method")
		}
		authConfig["key"] = cc.DestinationAPIKeyHeader

		// Where to send the key (header or query)
		authConfig["to"] = cc.DestinationAPIKeyTo

	case "custom_signature":
		// CUSTOM_SIGNATURE (SHA256 HMAC)
		if cc.DestinationCustomSignatureSecret == "" {
			return nil, fmt.Errorf("--destination-custom-signature-secret is required for custom_signature auth method")
		}
		if cc.DestinationCustomSignatureKey == "" {
			return nil, fmt.Errorf("--destination-custom-signature-key is required for custom_signature auth method")
		}
		authConfig["type"] = "CUSTOM_SIGNATURE"
		authConfig["signing_secret"] = cc.DestinationCustomSignatureSecret
		authConfig["key"] = cc.DestinationCustomSignatureKey

	case "oauth2_client_credentials":
		// OAUTH2_CLIENT_CREDENTIALS
		if cc.DestinationOAuth2AuthServer == "" {
			return nil, fmt.Errorf("--destination-oauth2-auth-server is required for oauth2_client_credentials auth method")
		}
		if cc.DestinationOAuth2ClientID == "" {
			return nil, fmt.Errorf("--destination-oauth2-client-id is required for oauth2_client_credentials auth method")
		}
		if cc.DestinationOAuth2ClientSecret == "" {
			return nil, fmt.Errorf("--destination-oauth2-client-secret is required for oauth2_client_credentials auth method")
		}

		authConfig["type"] = "OAUTH2_CLIENT_CREDENTIALS"
		authConfig["auth_server"] = cc.DestinationOAuth2AuthServer
		authConfig["client_id"] = cc.DestinationOAuth2ClientID
		authConfig["client_secret"] = cc.DestinationOAuth2ClientSecret

		if cc.DestinationOAuth2Scopes != "" {
			authConfig["scope"] = cc.DestinationOAuth2Scopes
		}
		if cc.DestinationOAuth2AuthType != "" {
			authConfig["authentication_type"] = cc.DestinationOAuth2AuthType
		}

	case "oauth2_authorization_code":
		// OAUTH2_AUTHORIZATION_CODE
		if cc.DestinationOAuth2AuthServer == "" {
			return nil, fmt.Errorf("--destination-oauth2-auth-server is required for oauth2_authorization_code auth method")
		}
		if cc.DestinationOAuth2ClientID == "" {
			return nil, fmt.Errorf("--destination-oauth2-client-id is required for oauth2_authorization_code auth method")
		}
		if cc.DestinationOAuth2ClientSecret == "" {
			return nil, fmt.Errorf("--destination-oauth2-client-secret is required for oauth2_authorization_code auth method")
		}
		if cc.DestinationOAuth2RefreshToken == "" {
			return nil, fmt.Errorf("--destination-oauth2-refresh-token is required for oauth2_authorization_code auth method")
		}

		authConfig["type"] = "OAUTH2_AUTHORIZATION_CODE"
		authConfig["auth_server"] = cc.DestinationOAuth2AuthServer
		authConfig["client_id"] = cc.DestinationOAuth2ClientID
		authConfig["client_secret"] = cc.DestinationOAuth2ClientSecret
		authConfig["refresh_token"] = cc.DestinationOAuth2RefreshToken

		if cc.DestinationOAuth2Scopes != "" {
			authConfig["scope"] = cc.DestinationOAuth2Scopes
		}

	case "aws":
		// AWS_SIGNATURE
		if cc.DestinationAWSAccessKeyID == "" {
			return nil, fmt.Errorf("--destination-aws-access-key-id is required for aws auth method")
		}
		if cc.DestinationAWSSecretAccessKey == "" {
			return nil, fmt.Errorf("--destination-aws-secret-access-key is required for aws auth method")
		}
		if cc.DestinationAWSRegion == "" {
			return nil, fmt.Errorf("--destination-aws-region is required for aws auth method")
		}
		if cc.DestinationAWSService == "" {
			return nil, fmt.Errorf("--destination-aws-service is required for aws auth method")
		}

		authConfig["type"] = "AWS_SIGNATURE"
		authConfig["access_key_id"] = cc.DestinationAWSAccessKeyID
		authConfig["secret_access_key"] = cc.DestinationAWSSecretAccessKey
		authConfig["region"] = cc.DestinationAWSRegion
		authConfig["service"] = cc.DestinationAWSService

	default:
		return nil, fmt.Errorf("unsupported destination authentication method: %s (supported: hookdeck, bearer, basic, api_key, custom_signature, oauth2_client_credentials, oauth2_authorization_code, aws)", cc.DestinationAuthMethod)
	}

	return authConfig, nil
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

	// Add allowed HTTP methods
	if cc.SourceAllowedHTTPMethods != "" {
		methods := strings.Split(cc.SourceAllowedHTTPMethods, ",")
		// Trim whitespace and validate
		validMethods := []string{}
		allowedMethods := map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}
		for _, method := range methods {
			method = strings.TrimSpace(strings.ToUpper(method))
			if !allowedMethods[method] {
				return nil, fmt.Errorf("invalid HTTP method '%s' in --source-allowed-http-methods (allowed: GET, POST, PUT, PATCH, DELETE)", method)
			}
			validMethods = append(validMethods, method)
		}
		config["allowed_http_methods"] = validMethods
	}

	// Add custom response configuration
	if cc.SourceCustomResponseType != "" || cc.SourceCustomResponseBody != "" {
		if cc.SourceCustomResponseType == "" {
			return nil, fmt.Errorf("--source-custom-response-content-type is required when using --source-custom-response-body")
		}
		if cc.SourceCustomResponseBody == "" {
			return nil, fmt.Errorf("--source-custom-response-body is required when using --source-custom-response-content-type")
		}

		// Validate content type
		validContentTypes := map[string]bool{"json": true, "text": true, "xml": true}
		contentType := strings.ToLower(cc.SourceCustomResponseType)
		if !validContentTypes[contentType] {
			return nil, fmt.Errorf("invalid content type '%s' in --source-custom-response-content-type (allowed: json, text, xml)", cc.SourceCustomResponseType)
		}

		// Validate body length (max 1000 chars per API spec)
		if len(cc.SourceCustomResponseBody) > 1000 {
			return nil, fmt.Errorf("--source-custom-response-body exceeds maximum length of 1000 characters (got %d)", len(cc.SourceCustomResponseBody))
		}

		config["custom_response"] = map[string]interface{}{
			"content_type": contentType,
			"body":         cc.SourceCustomResponseBody,
		}
	}

	if len(config) == 0 {
		return make(map[string]interface{}), nil
	}

	return config, nil
}

// buildRulesArray constructs the rules array from flags in logical execution order
// Order: filter -> transform -> deduplicate -> delay -> retry
// Note: This is the default order for individual flags. For custom order, use --rules or --rules-file
func (cc *connectionCreateCmd) buildRulesArray(cmd *cobra.Command) ([]hookdeck.Rule, error) {
	// Handle JSON fallback first
	if cc.Rules != "" {
		var rules []hookdeck.Rule
		if err := json.Unmarshal([]byte(cc.Rules), &rules); err != nil {
			return nil, fmt.Errorf("invalid JSON in --rules: %w", err)
		}
		return rules, nil
	}
	if cc.RulesFile != "" {
		data, err := os.ReadFile(cc.RulesFile)
		if err != nil {
			return nil, fmt.Errorf("could not read --rules-file: %w", err)
		}
		var rules []hookdeck.Rule
		if err := json.Unmarshal(data, &rules); err != nil {
			return nil, fmt.Errorf("invalid JSON in --rules-file: %w", err)
		}
		return rules, nil
	}

	// Track which rule types have been encountered
	ruleMap := make(map[string]hookdeck.Rule)

	// Determine which rule types are present by checking flags
	// Note: We don't track order from flags because pflag.Visit() processes flags alphabetically
	hasRetryFlags := cc.RuleRetryStrategy != "" || cc.RuleRetryCount > 0 || cc.RuleRetryInterval > 0 || cc.RuleRetryResponseStatusCode != ""
	hasFilterFlags := cc.RuleFilterBody != "" || cc.RuleFilterHeaders != "" || cc.RuleFilterQuery != "" || cc.RuleFilterPath != ""
	hasTransformFlags := cc.RuleTransformName != "" || cc.RuleTransformCode != "" || cc.RuleTransformEnv != ""
	hasDelayFlags := cc.RuleDelay > 0
	hasDeduplicateFlags := cc.RuleDeduplicateWindow > 0 || cc.RuleDeduplicateIncludeFields != "" || cc.RuleDeduplicateExcludeFields != ""

	// Initialize rule entries for each type that has flags set
	if hasRetryFlags {
		ruleMap["retry"] = make(hookdeck.Rule)
	}
	if hasFilterFlags {
		ruleMap["filter"] = make(hookdeck.Rule)
	}
	if hasTransformFlags {
		ruleMap["transform"] = make(hookdeck.Rule)
	}
	if hasDelayFlags {
		ruleMap["delay"] = make(hookdeck.Rule)
	}
	if hasDeduplicateFlags {
		ruleMap["deduplicate"] = make(hookdeck.Rule)
	}

	// Build each rule based on the flags set
	if rule, ok := ruleMap["retry"]; ok {
		rule["type"] = "retry"
		if cc.RuleRetryStrategy != "" {
			rule["strategy"] = cc.RuleRetryStrategy
		}
		if cc.RuleRetryCount > 0 {
			rule["count"] = cc.RuleRetryCount
		}
		if cc.RuleRetryInterval > 0 {
			rule["interval"] = cc.RuleRetryInterval
		}
		if cc.RuleRetryResponseStatusCode != "" {
			rule["response_status_codes"] = cc.RuleRetryResponseStatusCode
		}
	}

	if rule, ok := ruleMap["filter"]; ok {
		rule["type"] = "filter"
		if cc.RuleFilterBody != "" {
			rule["body"] = cc.RuleFilterBody
		}
		if cc.RuleFilterHeaders != "" {
			rule["headers"] = cc.RuleFilterHeaders
		}
		if cc.RuleFilterQuery != "" {
			rule["query"] = cc.RuleFilterQuery
		}
		if cc.RuleFilterPath != "" {
			rule["path"] = cc.RuleFilterPath
		}
	}

	if rule, ok := ruleMap["transform"]; ok {
		rule["type"] = "transform"
		transformConfig := make(map[string]interface{})
		if cc.RuleTransformName != "" {
			transformConfig["name"] = cc.RuleTransformName
		}
		if cc.RuleTransformCode != "" {
			transformConfig["code"] = cc.RuleTransformCode
		}
		if cc.RuleTransformEnv != "" {
			var env map[string]interface{}
			if err := json.Unmarshal([]byte(cc.RuleTransformEnv), &env); err != nil {
				return nil, fmt.Errorf("invalid JSON in --rule-transform-env: %w", err)
			}
			transformConfig["env"] = env
		}
		rule["transformation"] = transformConfig
	}

	if rule, ok := ruleMap["delay"]; ok {
		rule["type"] = "delay"
		if cc.RuleDelay > 0 {
			rule["delay"] = cc.RuleDelay
		}
	}

	if rule, ok := ruleMap["deduplicate"]; ok {
		rule["type"] = "deduplicate"
		if cc.RuleDeduplicateWindow > 0 {
			rule["window"] = cc.RuleDeduplicateWindow
		}
		if cc.RuleDeduplicateIncludeFields != "" {
			fields := strings.Split(cc.RuleDeduplicateIncludeFields, ",")
			rule["include_fields"] = fields
		}
		if cc.RuleDeduplicateExcludeFields != "" {
			fields := strings.Split(cc.RuleDeduplicateExcludeFields, ",")
			rule["exclude_fields"] = fields
		}
	}

	// Build rules array in logical execution order
	// Order: deduplicate -> transform -> filter -> delay -> retry
	// This order matches the API's default ordering for proper data flow through the pipeline
	rules := make([]hookdeck.Rule, 0, len(ruleMap))
	ruleTypes := []string{"deduplicate", "transform", "filter", "delay", "retry"}
	for _, ruleType := range ruleTypes {
		if rule, ok := ruleMap[ruleType]; ok {
			rules = append(rules, rule)
		}
	}

	return rules, nil
}

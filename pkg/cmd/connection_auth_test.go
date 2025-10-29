package cmd

import (
	"testing"
)

func TestBuildAuthConfig(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*connectionCreateCmd)
		wantType    string
		wantErr     bool
		errContains string
		validate    func(*testing.T, map[string]interface{})
	}{
		{
			name: "hookdeck signature explicit",
			setup: func(cc *connectionCreateCmd) {
				cc.DestinationAuthMethod = "hookdeck"
			},
			wantType: "HOOKDECK_SIGNATURE",
			wantErr:  false,
			validate: func(t *testing.T, config map[string]interface{}) {
				if config["type"] != "HOOKDECK_SIGNATURE" {
					t.Errorf("expected type HOOKDECK_SIGNATURE, got %v", config["type"])
				}
			},
		},
		{
			name: "empty auth method defaults to hookdeck",
			setup: func(cc *connectionCreateCmd) {
				cc.DestinationAuthMethod = ""
			},
			wantType: "",
			wantErr:  false,
			validate: func(t *testing.T, config map[string]interface{}) {
				if len(config) != 0 {
					t.Errorf("expected empty config for default auth, got %v", config)
				}
			},
		},
		{
			name: "bearer token valid",
			setup: func(cc *connectionCreateCmd) {
				cc.DestinationAuthMethod = "bearer"
				cc.DestinationBearerToken = "test-token-123"
			},
			wantType: "BEARER_TOKEN",
			wantErr:  false,
			validate: func(t *testing.T, config map[string]interface{}) {
				if config["type"] != "BEARER_TOKEN" {
					t.Errorf("expected type BEARER_TOKEN, got %v", config["type"])
				}
				if config["token"] != "test-token-123" {
					t.Errorf("expected token test-token-123, got %v", config["token"])
				}
			},
		},
		{
			name: "bearer token missing",
			setup: func(cc *connectionCreateCmd) {
				cc.DestinationAuthMethod = "bearer"
			},
			wantErr:     true,
			errContains: "--destination-bearer-token is required",
		},
		{
			name: "basic auth valid",
			setup: func(cc *connectionCreateCmd) {
				cc.DestinationAuthMethod = "basic"
				cc.DestinationBasicAuthUser = "testuser"
				cc.DestinationBasicAuthPass = "testpass"
			},
			wantType: "BASIC_AUTH",
			wantErr:  false,
			validate: func(t *testing.T, config map[string]interface{}) {
				if config["type"] != "BASIC_AUTH" {
					t.Errorf("expected type BASIC_AUTH, got %v", config["type"])
				}
				if config["username"] != "testuser" {
					t.Errorf("expected username testuser, got %v", config["username"])
				}
				if config["password"] != "testpass" {
					t.Errorf("expected password testpass, got %v", config["password"])
				}
			},
		},
		{
			name: "basic auth missing username",
			setup: func(cc *connectionCreateCmd) {
				cc.DestinationAuthMethod = "basic"
				cc.DestinationBasicAuthPass = "testpass"
			},
			wantErr:     true,
			errContains: "--destination-basic-auth-user and --destination-basic-auth-pass are required",
		},
		{
			name: "api key valid with header",
			setup: func(cc *connectionCreateCmd) {
				cc.DestinationAuthMethod = "api_key"
				cc.DestinationAPIKey = "sk_test_123"
				cc.DestinationAPIKeyHeader = "X-API-Key"
				cc.DestinationAPIKeyTo = "header"
			},
			wantType: "API_KEY",
			wantErr:  false,
			validate: func(t *testing.T, config map[string]interface{}) {
				if config["type"] != "API_KEY" {
					t.Errorf("expected type API_KEY, got %v", config["type"])
				}
				if config["api_key"] != "sk_test_123" {
					t.Errorf("expected api_key sk_test_123, got %v", config["api_key"])
				}
				if config["key"] != "X-API-Key" {
					t.Errorf("expected key X-API-Key, got %v", config["key"])
				}
				if config["to"] != "header" {
					t.Errorf("expected to header, got %v", config["to"])
				}
			},
		},
		{
			name: "api key valid with query",
			setup: func(cc *connectionCreateCmd) {
				cc.DestinationAuthMethod = "api_key"
				cc.DestinationAPIKey = "sk_test_123"
				cc.DestinationAPIKeyHeader = "api_key"
				cc.DestinationAPIKeyTo = "query"
			},
			wantType: "API_KEY",
			wantErr:  false,
			validate: func(t *testing.T, config map[string]interface{}) {
				if config["to"] != "query" {
					t.Errorf("expected to query, got %v", config["to"])
				}
			},
		},
		{
			name: "api key missing key",
			setup: func(cc *connectionCreateCmd) {
				cc.DestinationAuthMethod = "api_key"
				cc.DestinationAPIKeyHeader = "X-API-Key"
			},
			wantErr:     true,
			errContains: "--destination-api-key is required",
		},
		{
			name: "api key missing header",
			setup: func(cc *connectionCreateCmd) {
				cc.DestinationAuthMethod = "api_key"
				cc.DestinationAPIKey = "sk_test_123"
			},
			wantErr:     true,
			errContains: "--destination-api-key-header is required",
		},
		{
			name: "custom signature valid",
			setup: func(cc *connectionCreateCmd) {
				cc.DestinationAuthMethod = "custom_signature"
				cc.DestinationCustomSignatureKey = "X-Signature"
				cc.DestinationCustomSignatureSecret = "secret123"
			},
			wantType: "CUSTOM_SIGNATURE",
			wantErr:  false,
			validate: func(t *testing.T, config map[string]interface{}) {
				if config["type"] != "CUSTOM_SIGNATURE" {
					t.Errorf("expected type CUSTOM_SIGNATURE, got %v", config["type"])
				}
				if config["key"] != "X-Signature" {
					t.Errorf("expected key X-Signature, got %v", config["key"])
				}
				if config["signing_secret"] != "secret123" {
					t.Errorf("expected signing_secret secret123, got %v", config["signing_secret"])
				}
			},
		},
		{
			name: "custom signature missing secret",
			setup: func(cc *connectionCreateCmd) {
				cc.DestinationAuthMethod = "custom_signature"
				cc.DestinationCustomSignatureKey = "X-Signature"
			},
			wantErr:     true,
			errContains: "--destination-custom-signature-secret is required",
		},
		{
			name: "oauth2 client credentials valid",
			setup: func(cc *connectionCreateCmd) {
				cc.DestinationAuthMethod = "oauth2_client_credentials"
				cc.DestinationOAuth2AuthServer = "https://auth.example.com/token"
				cc.DestinationOAuth2ClientID = "client123"
				cc.DestinationOAuth2ClientSecret = "secret456"
				cc.DestinationOAuth2Scopes = "read write"
				cc.DestinationOAuth2AuthType = "basic"
			},
			wantType: "OAUTH2_CLIENT_CREDENTIALS",
			wantErr:  false,
			validate: func(t *testing.T, config map[string]interface{}) {
				if config["type"] != "OAUTH2_CLIENT_CREDENTIALS" {
					t.Errorf("expected type OAUTH2_CLIENT_CREDENTIALS, got %v", config["type"])
				}
				if config["auth_server"] != "https://auth.example.com/token" {
					t.Errorf("expected auth_server URL, got %v", config["auth_server"])
				}
				if config["client_id"] != "client123" {
					t.Errorf("expected client_id client123, got %v", config["client_id"])
				}
				if config["client_secret"] != "secret456" {
					t.Errorf("expected client_secret secret456, got %v", config["client_secret"])
				}
				if config["scope"] != "read write" {
					t.Errorf("expected scope 'read write', got %v", config["scope"])
				}
				if config["authentication_type"] != "basic" {
					t.Errorf("expected authentication_type basic, got %v", config["authentication_type"])
				}
			},
		},
		{
			name: "oauth2 client credentials missing auth server",
			setup: func(cc *connectionCreateCmd) {
				cc.DestinationAuthMethod = "oauth2_client_credentials"
				cc.DestinationOAuth2ClientID = "client123"
				cc.DestinationOAuth2ClientSecret = "secret456"
			},
			wantErr:     true,
			errContains: "--destination-oauth2-auth-server is required",
		},
		{
			name: "oauth2 authorization code valid",
			setup: func(cc *connectionCreateCmd) {
				cc.DestinationAuthMethod = "oauth2_authorization_code"
				cc.DestinationOAuth2AuthServer = "https://auth.example.com/token"
				cc.DestinationOAuth2ClientID = "client123"
				cc.DestinationOAuth2ClientSecret = "secret456"
				cc.DestinationOAuth2RefreshToken = "refresh789"
				cc.DestinationOAuth2Scopes = "read write"
			},
			wantType: "OAUTH2_AUTHORIZATION_CODE",
			wantErr:  false,
			validate: func(t *testing.T, config map[string]interface{}) {
				if config["type"] != "OAUTH2_AUTHORIZATION_CODE" {
					t.Errorf("expected type OAUTH2_AUTHORIZATION_CODE, got %v", config["type"])
				}
				if config["refresh_token"] != "refresh789" {
					t.Errorf("expected refresh_token refresh789, got %v", config["refresh_token"])
				}
			},
		},
		{
			name: "oauth2 authorization code missing refresh token",
			setup: func(cc *connectionCreateCmd) {
				cc.DestinationAuthMethod = "oauth2_authorization_code"
				cc.DestinationOAuth2AuthServer = "https://auth.example.com/token"
				cc.DestinationOAuth2ClientID = "client123"
				cc.DestinationOAuth2ClientSecret = "secret456"
			},
			wantErr:     true,
			errContains: "--destination-oauth2-refresh-token is required",
		},
		{
			name: "aws signature valid",
			setup: func(cc *connectionCreateCmd) {
				cc.DestinationAuthMethod = "aws"
				cc.DestinationAWSAccessKeyID = "AKIAIOSFODNN7EXAMPLE"
				cc.DestinationAWSSecretAccessKey = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
				cc.DestinationAWSRegion = "us-east-1"
				cc.DestinationAWSService = "execute-api"
			},
			wantType: "AWS_SIGNATURE",
			wantErr:  false,
			validate: func(t *testing.T, config map[string]interface{}) {
				if config["type"] != "AWS_SIGNATURE" {
					t.Errorf("expected type AWS_SIGNATURE, got %v", config["type"])
				}
				if config["access_key_id"] != "AKIAIOSFODNN7EXAMPLE" {
					t.Errorf("expected access_key_id, got %v", config["access_key_id"])
				}
				if config["region"] != "us-east-1" {
					t.Errorf("expected region us-east-1, got %v", config["region"])
				}
				if config["service"] != "execute-api" {
					t.Errorf("expected service execute-api, got %v", config["service"])
				}
			},
		},
		{
			name: "aws signature missing region",
			setup: func(cc *connectionCreateCmd) {
				cc.DestinationAuthMethod = "aws"
				cc.DestinationAWSAccessKeyID = "AKIAIOSFODNN7EXAMPLE"
				cc.DestinationAWSSecretAccessKey = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
				cc.DestinationAWSService = "execute-api"
			},
			wantErr:     true,
			errContains: "--destination-aws-region is required",
		},
		{
			name: "unsupported auth method",
			setup: func(cc *connectionCreateCmd) {
				cc.DestinationAuthMethod = "invalid_method"
			},
			wantErr:     true,
			errContains: "unsupported destination authentication method",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := &connectionCreateCmd{}
			tt.setup(cc)

			config, err := cc.buildAuthConfig()

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing '%s', got nil", tt.errContains)
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.validate != nil {
				tt.validate(t, config)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

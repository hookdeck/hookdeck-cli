package cmd

import (
	"testing"
)

func TestBuildSourceConfig(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*connectionCreateCmd)
		wantErr     bool
		errContains string
		validate    func(*testing.T, map[string]interface{})
	}{
		{
			name: "webhook secret auth",
			setup: func(cc *connectionCreateCmd) {
				cc.SourceWebhookSecret = "whsec_test123"
			},
			wantErr: false,
			validate: func(t *testing.T, config map[string]interface{}) {
				auth, ok := config["auth"].(map[string]interface{})
				if !ok {
					t.Errorf("expected auth map, got %T", config["auth"])
					return
				}
				if auth["webhook_secret_key"] != "whsec_test123" {
					t.Errorf("expected auth.webhook_secret_key whsec_test123, got %v", auth["webhook_secret_key"])
				}
			},
		},
		{
			name: "api key auth",
			setup: func(cc *connectionCreateCmd) {
				cc.SourceAPIKey = "sk_test_abc123"
			},
			wantErr: false,
			validate: func(t *testing.T, config map[string]interface{}) {
				auth, ok := config["auth"].(map[string]interface{})
				if !ok {
					t.Errorf("expected auth map, got %T", config["auth"])
					return
				}
				if auth["api_key"] != "sk_test_abc123" {
					t.Errorf("expected auth.api_key sk_test_abc123, got %v", auth["api_key"])
				}
			},
		},
		{
			name: "basic auth",
			setup: func(cc *connectionCreateCmd) {
				cc.SourceBasicAuthUser = "testuser"
				cc.SourceBasicAuthPass = "testpass"
			},
			wantErr: false,
			validate: func(t *testing.T, config map[string]interface{}) {
				auth, ok := config["auth"].(map[string]interface{})
				if !ok {
					t.Errorf("expected auth map, got %T", config["auth"])
					return
				}
				if auth["username"] != "testuser" {
					t.Errorf("expected auth.username testuser, got %v", auth["username"])
				}
				if auth["password"] != "testpass" {
					t.Errorf("expected auth.password testpass, got %v", auth["password"])
				}
			},
		},
		{
			name: "hmac auth with algorithm",
			setup: func(cc *connectionCreateCmd) {
				cc.SourceHMACSecret = "secret123"
				cc.SourceHMACAlgo = "SHA256"
			},
			wantErr: false,
			validate: func(t *testing.T, config map[string]interface{}) {
				auth, ok := config["auth"].(map[string]interface{})
				if !ok {
					t.Errorf("expected auth map, got %T", config["auth"])
					return
				}
				if auth["webhook_secret_key"] != "secret123" {
					t.Errorf("expected auth.webhook_secret_key secret123, got %v", auth["webhook_secret_key"])
				}
				if auth["algorithm"] != "sha256" {
					t.Errorf("expected auth.algorithm sha256, got %v", auth["algorithm"])
				}
			},
		},
		{
			name: "hmac auth without algorithm",
			setup: func(cc *connectionCreateCmd) {
				cc.SourceHMACSecret = "secret123"
			},
			wantErr: false,
			validate: func(t *testing.T, config map[string]interface{}) {
				auth, ok := config["auth"].(map[string]interface{})
				if !ok {
					t.Errorf("expected auth map, got %T", config["auth"])
					return
				}
				if auth["webhook_secret_key"] != "secret123" {
					t.Errorf("expected auth.webhook_secret_key secret123, got %v", auth["webhook_secret_key"])
				}
				if auth["algorithm"] != "sha256" {
					t.Errorf("expected default auth.algorithm sha256, got %v", auth["algorithm"])
				}
			},
		},
		{
			name: "allowed http methods - single method",
			setup: func(cc *connectionCreateCmd) {
				cc.SourceAllowedHTTPMethods = "POST"
			},
			wantErr: false,
			validate: func(t *testing.T, config map[string]interface{}) {
				methods, ok := config["allowed_http_methods"].([]string)
				if !ok {
					t.Errorf("expected allowed_http_methods []string, got %T", config["allowed_http_methods"])
					return
				}
				if len(methods) != 1 || methods[0] != "POST" {
					t.Errorf("expected [POST], got %v", methods)
				}
			},
		},
		{
			name: "allowed http methods - multiple methods",
			setup: func(cc *connectionCreateCmd) {
				cc.SourceAllowedHTTPMethods = "POST,PUT,PATCH,DELETE"
			},
			wantErr: false,
			validate: func(t *testing.T, config map[string]interface{}) {
				methods, ok := config["allowed_http_methods"].([]string)
				if !ok {
					t.Errorf("expected allowed_http_methods []string, got %T", config["allowed_http_methods"])
					return
				}
				if len(methods) != 4 {
					t.Errorf("expected 4 methods, got %d", len(methods))
				}
				expectedMethods := []string{"POST", "PUT", "PATCH", "DELETE"}
				for i, expected := range expectedMethods {
					if methods[i] != expected {
						t.Errorf("expected method[%d] to be %s, got %s", i, expected, methods[i])
					}
				}
			},
		},
		{
			name: "allowed http methods - with whitespace",
			setup: func(cc *connectionCreateCmd) {
				cc.SourceAllowedHTTPMethods = " POST , PUT , PATCH "
			},
			wantErr: false,
			validate: func(t *testing.T, config map[string]interface{}) {
				methods, ok := config["allowed_http_methods"].([]string)
				if !ok {
					t.Errorf("expected allowed_http_methods []string, got %T", config["allowed_http_methods"])
					return
				}
				if len(methods) != 3 || methods[0] != "POST" || methods[1] != "PUT" || methods[2] != "PATCH" {
					t.Errorf("expected [POST PUT PATCH], got %v", methods)
				}
			},
		},
		{
			name: "allowed http methods - lowercase converted to uppercase",
			setup: func(cc *connectionCreateCmd) {
				cc.SourceAllowedHTTPMethods = "post,get"
			},
			wantErr: false,
			validate: func(t *testing.T, config map[string]interface{}) {
				methods, ok := config["allowed_http_methods"].([]string)
				if !ok {
					t.Errorf("expected allowed_http_methods []string, got %T", config["allowed_http_methods"])
					return
				}
				if len(methods) != 2 || methods[0] != "POST" || methods[1] != "GET" {
					t.Errorf("expected [POST GET], got %v", methods)
				}
			},
		},
		{
			name: "allowed http methods - invalid method",
			setup: func(cc *connectionCreateCmd) {
				cc.SourceAllowedHTTPMethods = "POST,INVALID"
			},
			wantErr:     true,
			errContains: "invalid HTTP method",
		},
		{
			name: "custom response - json content type",
			setup: func(cc *connectionCreateCmd) {
				cc.SourceCustomResponseType = "json"
				cc.SourceCustomResponseBody = `{"status":"received"}`
			},
			wantErr: false,
			validate: func(t *testing.T, config map[string]interface{}) {
				customResp, ok := config["custom_response"].(map[string]interface{})
				if !ok {
					t.Errorf("expected custom_response map, got %T", config["custom_response"])
					return
				}
				if customResp["content_type"] != "json" {
					t.Errorf("expected content_type json, got %v", customResp["content_type"])
				}
				if customResp["body"] != `{"status":"received"}` {
					t.Errorf("expected body {\"status\":\"received\"}, got %v", customResp["body"])
				}
			},
		},
		{
			name: "custom response - text content type",
			setup: func(cc *connectionCreateCmd) {
				cc.SourceCustomResponseType = "text"
				cc.SourceCustomResponseBody = "OK"
			},
			wantErr: false,
			validate: func(t *testing.T, config map[string]interface{}) {
				customResp, ok := config["custom_response"].(map[string]interface{})
				if !ok {
					t.Errorf("expected custom_response map, got %T", config["custom_response"])
					return
				}
				if customResp["content_type"] != "text" {
					t.Errorf("expected content_type text, got %v", customResp["content_type"])
				}
				if customResp["body"] != "OK" {
					t.Errorf("expected body OK, got %v", customResp["body"])
				}
			},
		},
		{
			name: "custom response - xml content type",
			setup: func(cc *connectionCreateCmd) {
				cc.SourceCustomResponseType = "xml"
				cc.SourceCustomResponseBody = `<?xml version="1.0"?><status>received</status>`
			},
			wantErr: false,
			validate: func(t *testing.T, config map[string]interface{}) {
				customResp, ok := config["custom_response"].(map[string]interface{})
				if !ok {
					t.Errorf("expected custom_response map, got %T", config["custom_response"])
					return
				}
				if customResp["content_type"] != "xml" {
					t.Errorf("expected content_type xml, got %v", customResp["content_type"])
				}
			},
		},
		{
			name: "custom response - uppercase content type normalized",
			setup: func(cc *connectionCreateCmd) {
				cc.SourceCustomResponseType = "JSON"
				cc.SourceCustomResponseBody = `{"status":"ok"}`
			},
			wantErr: false,
			validate: func(t *testing.T, config map[string]interface{}) {
				customResp, ok := config["custom_response"].(map[string]interface{})
				if !ok {
					t.Errorf("expected custom_response map, got %T", config["custom_response"])
					return
				}
				if customResp["content_type"] != "json" {
					t.Errorf("expected content_type json (normalized), got %v", customResp["content_type"])
				}
			},
		},
		{
			name: "custom response - missing body",
			setup: func(cc *connectionCreateCmd) {
				cc.SourceCustomResponseType = "json"
			},
			wantErr:     true,
			errContains: "--source-custom-response-body is required",
		},
		{
			name: "custom response - missing content type",
			setup: func(cc *connectionCreateCmd) {
				cc.SourceCustomResponseBody = `{"status":"received"}`
			},
			wantErr:     true,
			errContains: "--source-custom-response-content-type is required",
		},
		{
			name: "custom response - invalid content type",
			setup: func(cc *connectionCreateCmd) {
				cc.SourceCustomResponseType = "html"
				cc.SourceCustomResponseBody = "<html></html>"
			},
			wantErr:     true,
			errContains: "invalid content type",
		},
		{
			name: "custom response - body exceeds 1000 chars",
			setup: func(cc *connectionCreateCmd) {
				cc.SourceCustomResponseType = "text"
				// Create a body with 1001 characters
				body := ""
				for i := 0; i < 1001; i++ {
					body += "a"
				}
				cc.SourceCustomResponseBody = body
			},
			wantErr:     true,
			errContains: "exceeds maximum length of 1000 characters",
		},
		{
			name: "custom response - body exactly 1000 chars",
			setup: func(cc *connectionCreateCmd) {
				cc.SourceCustomResponseType = "text"
				// Create a body with exactly 1000 characters
				body := ""
				for i := 0; i < 1000; i++ {
					body += "a"
				}
				cc.SourceCustomResponseBody = body
			},
			wantErr: false,
			validate: func(t *testing.T, config map[string]interface{}) {
				customResp, ok := config["custom_response"].(map[string]interface{})
				if !ok {
					t.Errorf("expected custom_response map, got %T", config["custom_response"])
					return
				}
				body, ok := customResp["body"].(string)
				if !ok {
					t.Errorf("expected body string, got %T", customResp["body"])
					return
				}
				if len(body) != 1000 {
					t.Errorf("expected body length 1000, got %d", len(body))
				}
			},
		},
		{
			name: "combined - auth and allowed methods",
			setup: func(cc *connectionCreateCmd) {
				cc.SourceWebhookSecret = "whsec_123"
				cc.SourceAllowedHTTPMethods = "POST,PUT"
			},
			wantErr: false,
			validate: func(t *testing.T, config map[string]interface{}) {
				auth, _ := config["auth"].(map[string]interface{})
				if auth == nil || auth["webhook_secret_key"] != "whsec_123" {
					t.Errorf("expected auth.webhook_secret_key whsec_123, got %v", config["auth"])
				}
				methods, ok := config["allowed_http_methods"].([]string)
				if !ok || len(methods) != 2 {
					t.Errorf("expected 2 methods, got %v", config["allowed_http_methods"])
				}
			},
		},
		{
			name: "combined - auth and custom response",
			setup: func(cc *connectionCreateCmd) {
				cc.SourceAPIKey = "sk_test_123"
				cc.SourceCustomResponseType = "json"
				cc.SourceCustomResponseBody = `{"ok":true}`
			},
			wantErr: false,
			validate: func(t *testing.T, config map[string]interface{}) {
				auth, _ := config["auth"].(map[string]interface{})
				if auth == nil || auth["api_key"] != "sk_test_123" {
					t.Errorf("expected auth.api_key sk_test_123, got %v", config["auth"])
				}
				if config["custom_response"] == nil {
					t.Errorf("expected custom_response to be set")
				}
			},
		},
		{
			name: "combined - all source config options",
			setup: func(cc *connectionCreateCmd) {
				cc.SourceWebhookSecret = "whsec_123"
				cc.SourceAllowedHTTPMethods = "POST,PUT,DELETE"
				cc.SourceCustomResponseType = "json"
				cc.SourceCustomResponseBody = `{"status":"ok"}`
			},
			wantErr: false,
			validate: func(t *testing.T, config map[string]interface{}) {
				auth, _ := config["auth"].(map[string]interface{})
				if auth == nil || auth["webhook_secret_key"] != "whsec_123" {
					t.Errorf("expected auth.webhook_secret_key whsec_123")
				}
				if config["allowed_http_methods"] == nil {
					t.Errorf("expected allowed_http_methods")
				}
				if config["custom_response"] == nil {
					t.Errorf("expected custom_response")
				}
			},
		},
		{
			name: "empty config",
			setup: func(cc *connectionCreateCmd) {
				// No flags set
			},
			wantErr: false,
			validate: func(t *testing.T, config map[string]interface{}) {
				if len(config) != 0 {
					t.Errorf("expected empty config, got %v", config)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := &connectionCreateCmd{}
			tt.setup(cc)

			config, err := cc.buildSourceConfig()

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

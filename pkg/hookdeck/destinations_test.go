package hookdeck

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestCreateDestination(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		request        DestinationCreateRequest
		mockResponse   Destination
		mockStatusCode int
		wantErr        bool
		errContains    string
	}{
		{
			name: "successful create with HTTP destination",
			request: DestinationCreateRequest{
				Name:        "test-destination",
				Description: stringPtr("test description"),
				URL:         stringPtr("https://api.example.com/webhooks"),
				Config:      map[string]interface{}{},
			},
			mockResponse: Destination{
				ID:          "dest_123",
				Name:        "test-destination",
				Description: stringPtr("test description"),
				URL:         stringPtr("https://api.example.com/webhooks"),
				Type:        "HTTP",
				Config:      map[string]interface{}{},
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name: "successful create with CLI destination",
			request: DestinationCreateRequest{
				Name:        "cli-destination",
				Description: stringPtr("CLI destination for local testing"),
				CliPath:     stringPtr("/webhooks"),
				Config:      map[string]interface{}{},
			},
			mockResponse: Destination{
				ID:          "dest_456",
				Name:        "cli-destination",
				Description: stringPtr("CLI destination for local testing"),
				Type:        "CLI",
				CliPath:     stringPtr("/webhooks"),
				Config:      map[string]interface{}{},
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name: "successful create with authentication config",
			request: DestinationCreateRequest{
				Name:        "secure-destination",
				Description: stringPtr("Destination with API key auth"),
				URL:         stringPtr("https://api.example.com/webhooks"),
				Config: map[string]interface{}{
					"auth": map[string]interface{}{
						"type":    "api_key",
						"api_key": "secret-key-123",
					},
				},
			},
			mockResponse: Destination{
				ID:          "dest_789",
				Name:        "secure-destination",
				Description: stringPtr("Destination with API key auth"),
				URL:         stringPtr("https://api.example.com/webhooks"),
				Type:        "HTTP",
				Config: map[string]interface{}{
					"auth": map[string]interface{}{
						"type":    "api_key",
						"api_key": "secret-key-123",
					},
				},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name: "bad request error - missing name",
			request: DestinationCreateRequest{
				Name: "",
			},
			mockStatusCode: http.StatusBadRequest,
			wantErr:        true,
			errContains:    "400",
		},
		{
			name: "server error",
			request: DestinationCreateRequest{
				Name: "test-destination",
			},
			mockStatusCode: http.StatusInternalServerError,
			wantErr:        true,
			errContains:    "500",
		},
		{
			name: "conflict error - duplicate name",
			request: DestinationCreateRequest{
				Name: "existing-destination",
			},
			mockStatusCode: http.StatusConflict,
			wantErr:        true,
			errContains:    "409",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("expected POST request, got %s", r.Method)
				}
				if r.URL.Path != "/2024-03-01/destinations" {
					t.Errorf("expected path /2024-03-01/destinations, got %s", r.URL.Path)
				}

				// Verify request body
				var receivedReq DestinationCreateRequest
				if err := json.NewDecoder(r.Body).Decode(&receivedReq); err != nil {
					t.Fatalf("failed to decode request body: %v", err)
				}

				if receivedReq.Name != tt.request.Name {
					t.Errorf("expected name %s, got %s", tt.request.Name, receivedReq.Name)
				}

				w.WriteHeader(tt.mockStatusCode)
				if tt.mockStatusCode == http.StatusOK {
					json.NewEncoder(w).Encode(tt.mockResponse)
				} else {
					json.NewEncoder(w).Encode(ErrorResponse{
						Message: "test error",
					})
				}
			})
			defer server.Close()

			result, err := client.CreateDestination(context.Background(), &tt.request)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.ID != tt.mockResponse.ID {
				t.Errorf("expected ID %s, got %s", tt.mockResponse.ID, result.ID)
			}

			if result.Name != tt.mockResponse.Name {
				t.Errorf("expected name %s, got %s", tt.mockResponse.Name, result.Name)
			}

			if result.Type != tt.mockResponse.Type {
				t.Errorf("expected type %s, got %s", tt.mockResponse.Type, result.Type)
			}
		})
	}
}

func TestCreateDestination_Marshaling(t *testing.T) {
	t.Parallel()

	t.Run("verify request marshaling", func(t *testing.T) {
		req := DestinationCreateRequest{
			Name:        "test-destination",
			Description: stringPtr("test description"),
			URL:         stringPtr("https://api.example.com/webhooks"),
			Config: map[string]interface{}{
				"auth": map[string]interface{}{
					"type":    "api_key",
					"api_key": "secret123",
				},
			},
		}

		data, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("failed to marshal request: %v", err)
		}

		var unmarshaled DestinationCreateRequest
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}

		if unmarshaled.Name != req.Name {
			t.Errorf("expected name %s, got %s", req.Name, unmarshaled.Name)
		}

		if *unmarshaled.Description != *req.Description {
			t.Errorf("expected description %s, got %s", *req.Description, *unmarshaled.Description)
		}

		if *unmarshaled.URL != *req.URL {
			t.Errorf("expected URL %s, got %s", *req.URL, *unmarshaled.URL)
		}
	})
}

func TestCreateDestination_ConfigVariations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config map[string]interface{}
	}{
		{
			name:   "empty config",
			config: map[string]interface{}{},
		},
		{
			name: "api key auth config",
			config: map[string]interface{}{
				"auth": map[string]interface{}{
					"type":    "api_key",
					"api_key": "secret-key",
				},
			},
		},
		{
			name: "basic auth config",
			config: map[string]interface{}{
				"auth": map[string]interface{}{
					"type":     "basic",
					"username": "user",
					"password": "pass",
				},
			},
		},
		{
			name: "bearer token auth config",
			config: map[string]interface{}{
				"auth": map[string]interface{}{
					"type":  "bearer",
					"token": "bearer-token-123",
				},
			},
		},
		{
			name: "custom headers config",
			config: map[string]interface{}{
				"headers": map[string]interface{}{
					"X-Custom-Header": "custom-value",
					"X-API-Version":   "v1",
				},
			},
		},
		{
			name: "rate limit config",
			config: map[string]interface{}{
				"rate_limit": map[string]interface{}{
					"limit":  100,
					"period": "minute",
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
				var receivedReq DestinationCreateRequest
				if err := json.NewDecoder(r.Body).Decode(&receivedReq); err != nil {
					t.Fatalf("failed to decode request body: %v", err)
				}

				// Verify config was sent correctly
				if len(receivedReq.Config) != len(tt.config) {
					t.Errorf("expected config length %d, got %d", len(tt.config), len(receivedReq.Config))
				}

				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(Destination{
					ID:        "dest_123",
					Name:      "test-destination",
					Type:      "HTTP",
					Config:    tt.config,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				})
			})
			defer server.Close()

			req := &DestinationCreateRequest{
				Name:   "test-destination",
				Config: tt.config,
			}

			result, err := client.CreateDestination(context.Background(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result.Config) != len(tt.config) {
				t.Errorf("expected config length %d, got %d", len(tt.config), len(result.Config))
			}
		})
	}
}

func TestCreateDestination_URLandCliPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		url     *string
		cliPath *string
	}{
		{
			name:    "HTTP destination with URL",
			url:     stringPtr("https://api.example.com/webhooks"),
			cliPath: nil,
		},
		{
			name:    "CLI destination with path",
			url:     nil,
			cliPath: stringPtr("/webhooks"),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
				var receivedReq DestinationCreateRequest
				if err := json.NewDecoder(r.Body).Decode(&receivedReq); err != nil {
					t.Fatalf("failed to decode request body: %v", err)
				}

				// Verify URL or CliPath
				if tt.url != nil && (receivedReq.URL == nil || *receivedReq.URL != *tt.url) {
					t.Errorf("expected URL %v, got %v", tt.url, receivedReq.URL)
				}
				if tt.cliPath != nil && (receivedReq.CliPath == nil || *receivedReq.CliPath != *tt.cliPath) {
					t.Errorf("expected CliPath %v, got %v", tt.cliPath, receivedReq.CliPath)
				}

				w.WriteHeader(http.StatusOK)
				destType := "HTTP"
				if tt.cliPath != nil {
					destType = "CLI"
				}
				json.NewEncoder(w).Encode(Destination{
					ID:        "dest_123",
					Name:      "test-destination",
					Type:      destType,
					URL:       tt.url,
					CliPath:   tt.cliPath,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				})
			})
			defer server.Close()

			req := &DestinationCreateRequest{
				Name:    "test-destination",
				URL:     tt.url,
				CliPath: tt.cliPath,
			}

			result, err := client.CreateDestination(context.Background(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.url != nil && (result.URL == nil || *result.URL != *tt.url) {
				t.Errorf("expected URL %v, got %v", tt.url, result.URL)
			}

			if tt.cliPath != nil && (result.CliPath == nil || *result.CliPath != *tt.cliPath) {
				t.Errorf("expected CliPath %v, got %v", tt.cliPath, result.CliPath)
			}
		})
	}
}

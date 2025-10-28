package hookdeck

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestCreateSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		request        SourceCreateRequest
		mockResponse   Source
		mockStatusCode int
		wantErr        bool
		errContains    string
	}{
		{
			name: "successful create with basic config",
			request: SourceCreateRequest{
				Name:        "test-source",
				Description: stringPtr("test description"),
				Config:      map[string]interface{}{},
			},
			mockResponse: Source{
				ID:          "src_123",
				Name:        "test-source",
				Description: stringPtr("test description"),
				URL:         "https://events.hookdeck.com/e/src_123",
				Type:        "WEBHOOK",
				Config:      map[string]interface{}{},
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name: "successful create with webhook secret",
			request: SourceCreateRequest{
				Name:        "stripe-source",
				Description: stringPtr("Stripe webhook source"),
				Config: map[string]interface{}{
					"webhook_secret": "whsec_test123",
				},
			},
			mockResponse: Source{
				ID:          "src_456",
				Name:        "stripe-source",
				Description: stringPtr("Stripe webhook source"),
				URL:         "https://events.hookdeck.com/e/src_456",
				Type:        "WEBHOOK",
				Config: map[string]interface{}{
					"webhook_secret": "whsec_test123",
				},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name: "bad request error - missing name",
			request: SourceCreateRequest{
				Name: "",
			},
			mockStatusCode: http.StatusBadRequest,
			wantErr:        true,
			errContains:    "400",
		},
		{
			name: "server error",
			request: SourceCreateRequest{
				Name: "test-source",
			},
			mockStatusCode: http.StatusInternalServerError,
			wantErr:        true,
			errContains:    "500",
		},
		{
			name: "conflict error - duplicate name",
			request: SourceCreateRequest{
				Name: "existing-source",
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
				if r.URL.Path != "/2024-03-01/sources" {
					t.Errorf("expected path /2024-03-01/sources, got %s", r.URL.Path)
				}

				// Verify request body
				var receivedReq SourceCreateRequest
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

			result, err := client.CreateSource(context.Background(), &tt.request)

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

func TestCreateSource_Marshaling(t *testing.T) {
	t.Parallel()

	t.Run("verify request marshaling", func(t *testing.T) {
		req := SourceCreateRequest{
			Name:        "test-source",
			Description: stringPtr("test description"),
			Config: map[string]interface{}{
				"webhook_secret": "secret123",
				"verification":   map[string]interface{}{"enabled": true},
			},
		}

		data, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("failed to marshal request: %v", err)
		}

		var unmarshaled SourceCreateRequest
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}

		if unmarshaled.Name != req.Name {
			t.Errorf("expected name %s, got %s", req.Name, unmarshaled.Name)
		}

		if *unmarshaled.Description != *req.Description {
			t.Errorf("expected description %s, got %s", *req.Description, *unmarshaled.Description)
		}
	})
}

func TestCreateSource_ConfigVariations(t *testing.T) {
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
			name: "webhook secret config",
			config: map[string]interface{}{
				"webhook_secret": "whsec_test123",
			},
		},
		{
			name: "verification config",
			config: map[string]interface{}{
				"verification": map[string]interface{}{
					"enabled": true,
					"type":    "stripe",
				},
			},
		},
		{
			name: "complex config",
			config: map[string]interface{}{
				"webhook_secret": "whsec_test123",
				"verification": map[string]interface{}{
					"enabled": true,
					"type":    "stripe",
				},
				"custom_response": map[string]interface{}{
					"status_code": 200,
					"body":        "OK",
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
				var receivedReq SourceCreateRequest
				if err := json.NewDecoder(r.Body).Decode(&receivedReq); err != nil {
					t.Fatalf("failed to decode request body: %v", err)
				}

				// Verify config was sent correctly
				if len(receivedReq.Config) != len(tt.config) {
					t.Errorf("expected config length %d, got %d", len(tt.config), len(receivedReq.Config))
				}

				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(Source{
					ID:        "src_123",
					Name:      "test-source",
					Type:      "WEBHOOK",
					Config:    tt.config,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				})
			})
			defer server.Close()

			req := &SourceCreateRequest{
				Name:   "test-source",
				Config: tt.config,
			}

			result, err := client.CreateSource(context.Background(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result.Config) != len(tt.config) {
				t.Errorf("expected config length %d, got %d", len(tt.config), len(result.Config))
			}
		})
	}
}

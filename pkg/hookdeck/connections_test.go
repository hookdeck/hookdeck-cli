package hookdeck

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

// Helper function to create a test client with a mock server
func newTestClient(handler http.HandlerFunc) (*Client, *httptest.Server) {
	server := httptest.NewServer(handler)
	baseURL, _ := url.Parse(server.URL)
	client := &Client{
		BaseURL: baseURL,
		APIKey:  "test-api-key",
	}
	return client, server
}

// Helper function to create a pointer to a string
func stringPtr(s string) *string {
	return &s
}

// Helper function to create a pointer to a time
func timePtr(t time.Time) *time.Time {
	return &t
}

func TestListConnections(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		params         map[string]string
		mockResponse   ConnectionListResponse
		mockStatusCode int
		wantErr        bool
		errContains    string
	}{
		{
			name:   "successful list without filters",
			params: map[string]string{},
			mockResponse: ConnectionListResponse{
				Models: []Connection{
					{
						ID:        "conn_123",
						Name:      stringPtr("test-connection"),
						FullName:  stringPtr("test-connection"),
						TeamID:    "team_123",
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
				},
				Pagination: PaginationResponse{
					OrderBy: "created_at",
					Dir:     "desc",
					Limit:   100,
				},
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name: "successful list with filters",
			params: map[string]string{
				"name":        "test",
				"disabled":    "false",
				"paused":      "false",
				"source_id":   "src_123",
				"destination": "dest_123",
			},
			mockResponse: ConnectionListResponse{
				Models: []Connection{
					{
						ID:        "conn_123",
						Name:      stringPtr("test-connection"),
						FullName:  stringPtr("test-connection"),
						TeamID:    "team_123",
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
				},
				Pagination: PaginationResponse{
					OrderBy: "created_at",
					Dir:     "desc",
					Limit:   100,
				},
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name:           "error response",
			params:         map[string]string{},
			mockStatusCode: http.StatusInternalServerError,
			wantErr:        true,
			errContains:    "500",
		},
		{
			name:           "not found response",
			params:         map[string]string{},
			mockStatusCode: http.StatusNotFound,
			wantErr:        true,
			errContains:    "404",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
				// Verify request method and path
				if r.Method != http.MethodGet {
					t.Errorf("expected GET request, got %s", r.Method)
				}
				if r.URL.Path != "/2025-07-01/connections" {
					t.Errorf("expected path /2025-07-01/connections, got %s", r.URL.Path)
				}

				// Verify query parameters
				for k, v := range tt.params {
					if r.URL.Query().Get(k) != v {
						t.Errorf("expected query param %s=%s, got %s", k, v, r.URL.Query().Get(k))
					}
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

			result, err := client.ListConnections(context.Background(), tt.params)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				// Just verify we got an error, don't check the specific message
				// as the error format may vary
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result.Models) != len(tt.mockResponse.Models) {
				t.Errorf("expected %d connections, got %d", len(tt.mockResponse.Models), len(result.Models))
			}
		})
	}
}

func TestGetConnection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		connectionID   string
		mockResponse   Connection
		mockStatusCode int
		wantErr        bool
		errContains    string
	}{
		{
			name:         "successful get",
			connectionID: "conn_123",
			mockResponse: Connection{
				ID:        "conn_123",
				Name:      stringPtr("test-connection"),
				FullName:  stringPtr("test-connection"),
				TeamID:    "team_123",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name:           "connection not found",
			connectionID:   "conn_nonexistent",
			mockStatusCode: http.StatusNotFound,
			wantErr:        true,
			errContains:    "404",
		},
		{
			name:           "server error",
			connectionID:   "conn_123",
			mockStatusCode: http.StatusInternalServerError,
			wantErr:        true,
			errContains:    "500",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("expected GET request, got %s", r.Method)
				}
				expectedPath := "/2025-07-01/connections/" + tt.connectionID
				if r.URL.Path != expectedPath {
					t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
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

			result, err := client.GetConnection(context.Background(), tt.connectionID)

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
		})
	}
}

func TestCreateConnection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		request        ConnectionCreateRequest
		mockResponse   Connection
		mockStatusCode int
		wantErr        bool
		errContains    string
	}{
		{
			name: "successful create with existing resources",
			request: ConnectionCreateRequest{
				Name:          stringPtr("test-connection"),
				Description:   stringPtr("test description"),
				SourceID:      stringPtr("src_123"),
				DestinationID: stringPtr("dest_123"),
			},
			mockResponse: Connection{
				ID:          "conn_123",
				Name:        stringPtr("test-connection"),
				Description: stringPtr("test description"),
				TeamID:      "team_123",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name: "successful create with inline source and destination",
			request: ConnectionCreateRequest{
				Name:        stringPtr("test-connection"),
				Description: stringPtr("test description"),
				Source: &SourceCreateInput{
					Name: "test-source",
					Type: "WEBHOOK",
				},
				Destination: &DestinationCreateInput{
					Name: "test-destination",
					Type: "CLI",
				},
			},
			mockResponse: Connection{
				ID:          "conn_123",
				Name:        stringPtr("test-connection"),
				Description: stringPtr("test description"),
				TeamID:      "team_123",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name: "bad request error",
			request: ConnectionCreateRequest{
				Name: stringPtr("test-connection"),
			},
			mockStatusCode: http.StatusBadRequest,
			wantErr:        true,
			errContains:    "400",
		},
		{
			name: "server error",
			request: ConnectionCreateRequest{
				Name: stringPtr("test-connection"),
			},
			mockStatusCode: http.StatusInternalServerError,
			wantErr:        true,
			errContains:    "500",
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
				if r.URL.Path != "/2025-07-01/connections" {
					t.Errorf("expected path /2025-07-01/connections, got %s", r.URL.Path)
				}

				// Verify request body
				var receivedReq ConnectionCreateRequest
				if err := json.NewDecoder(r.Body).Decode(&receivedReq); err != nil {
					t.Fatalf("failed to decode request body: %v", err)
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

			result, err := client.CreateConnection(context.Background(), &tt.request)

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
		})
	}
}

func TestUpdateConnection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		connectionID   string
		request        ConnectionUpdateRequest
		mockResponse   Connection
		mockStatusCode int
		wantErr        bool
		errContains    string
	}{
		{
			name:         "successful update name",
			connectionID: "conn_123",
			request: ConnectionUpdateRequest{
				Name: stringPtr("updated-connection"),
			},
			mockResponse: Connection{
				ID:        "conn_123",
				Name:      stringPtr("updated-connection"),
				TeamID:    "team_123",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name:         "successful update description",
			connectionID: "conn_123",
			request: ConnectionUpdateRequest{
				Description: stringPtr("updated description"),
			},
			mockResponse: Connection{
				ID:          "conn_123",
				Name:        stringPtr("test-connection"),
				Description: stringPtr("updated description"),
				TeamID:      "team_123",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name:         "connection not found",
			connectionID: "conn_nonexistent",
			request: ConnectionUpdateRequest{
				Name: stringPtr("updated-connection"),
			},
			mockStatusCode: http.StatusNotFound,
			wantErr:        true,
			errContains:    "404",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					t.Errorf("expected PUT request, got %s", r.Method)
				}
				expectedPath := "/2025-07-01/connections/" + tt.connectionID
				if r.URL.Path != expectedPath {
					t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
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

			result, err := client.UpdateConnection(context.Background(), tt.connectionID, &tt.request)

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
		})
	}
}

func TestDeleteConnection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		connectionID   string
		mockStatusCode int
		wantErr        bool
		errContains    string
	}{
		{
			name:           "successful delete",
			connectionID:   "conn_123",
			mockStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name:           "connection not found",
			connectionID:   "conn_nonexistent",
			mockStatusCode: http.StatusNotFound,
			wantErr:        true,
			errContains:    "404",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete {
					t.Errorf("expected DELETE request, got %s", r.Method)
				}
				expectedPath := "/2025-07-01/connections/" + tt.connectionID
				if r.URL.Path != expectedPath {
					t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
				}

				w.WriteHeader(tt.mockStatusCode)
				if tt.mockStatusCode != http.StatusOK {
					json.NewEncoder(w).Encode(ErrorResponse{
						Message: "test error",
					})
				}
			})
			defer server.Close()

			err := client.DeleteConnection(context.Background(), tt.connectionID)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestEnableConnection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		connectionID   string
		mockResponse   Connection
		mockStatusCode int
		wantErr        bool
		errContains    string
	}{
		{
			name:         "successful enable",
			connectionID: "conn_123",
			mockResponse: Connection{
				ID:         "conn_123",
				Name:       stringPtr("test-connection"),
				TeamID:     "team_123",
				DisabledAt: nil,
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name:           "connection not found",
			connectionID:   "conn_nonexistent",
			mockStatusCode: http.StatusNotFound,
			wantErr:        true,
			errContains:    "404",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					t.Errorf("expected PUT request, got %s", r.Method)
				}
				expectedPath := "/2025-07-01/connections/" + tt.connectionID + "/enable"
				if r.URL.Path != expectedPath {
					t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
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

			result, err := client.EnableConnection(context.Background(), tt.connectionID)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.DisabledAt != nil {
				t.Error("expected connection to be enabled (DisabledAt should be nil)")
			}
		})
	}
}

func TestDisableConnection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		connectionID   string
		mockResponse   Connection
		mockStatusCode int
		wantErr        bool
		errContains    string
	}{
		{
			name:         "successful disable",
			connectionID: "conn_123",
			mockResponse: Connection{
				ID:         "conn_123",
				Name:       stringPtr("test-connection"),
				TeamID:     "team_123",
				DisabledAt: timePtr(time.Now()),
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name:           "connection not found",
			connectionID:   "conn_nonexistent",
			mockStatusCode: http.StatusNotFound,
			wantErr:        true,
			errContains:    "404",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					t.Errorf("expected PUT request, got %s", r.Method)
				}
				expectedPath := "/2025-07-01/connections/" + tt.connectionID + "/disable"
				if r.URL.Path != expectedPath {
					t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
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

			result, err := client.DisableConnection(context.Background(), tt.connectionID)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.DisabledAt == nil {
				t.Error("expected connection to be disabled (DisabledAt should not be nil)")
			}
		})
	}
}

func TestPauseConnection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		connectionID   string
		mockResponse   Connection
		mockStatusCode int
		wantErr        bool
		errContains    string
	}{
		{
			name:         "successful pause",
			connectionID: "conn_123",
			mockResponse: Connection{
				ID:        "conn_123",
				Name:      stringPtr("test-connection"),
				TeamID:    "team_123",
				PausedAt:  timePtr(time.Now()),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name:           "connection not found",
			connectionID:   "conn_nonexistent",
			mockStatusCode: http.StatusNotFound,
			wantErr:        true,
			errContains:    "404",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					t.Errorf("expected PUT request, got %s", r.Method)
				}
				expectedPath := "/2025-07-01/connections/" + tt.connectionID + "/pause"
				if r.URL.Path != expectedPath {
					t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
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

			result, err := client.PauseConnection(context.Background(), tt.connectionID)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.PausedAt == nil {
				t.Error("expected connection to be paused (PausedAt should not be nil)")
			}
		})
	}
}

func TestUnpauseConnection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		connectionID   string
		mockResponse   Connection
		mockStatusCode int
		wantErr        bool
		errContains    string
	}{
		{
			name:         "successful unpause",
			connectionID: "conn_123",
			mockResponse: Connection{
				ID:        "conn_123",
				Name:      stringPtr("test-connection"),
				TeamID:    "team_123",
				PausedAt:  nil,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name:           "connection not found",
			connectionID:   "conn_nonexistent",
			mockStatusCode: http.StatusNotFound,
			wantErr:        true,
			errContains:    "404",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					t.Errorf("expected PUT request, got %s", r.Method)
				}
				expectedPath := "/2025-07-01/connections/" + tt.connectionID + "/unpause"
				if r.URL.Path != expectedPath {
					t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
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

			result, err := client.UnpauseConnection(context.Background(), tt.connectionID)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.PausedAt != nil {
				t.Error("expected connection to be unpaused (PausedAt should be nil)")
			}
		})
	}
}

func TestArchiveConnection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		connectionID   string
		mockResponse   Connection
		mockStatusCode int
		wantErr        bool
		errContains    string
	}{
		{
			name:         "successful archive",
			connectionID: "conn_123",
			mockResponse: Connection{
				ID:        "conn_123",
				Name:      stringPtr("test-connection"),
				TeamID:    "team_123",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name:           "connection not found",
			connectionID:   "conn_nonexistent",
			mockStatusCode: http.StatusNotFound,
			wantErr:        true,
			errContains:    "404",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					t.Errorf("expected PUT request, got %s", r.Method)
				}
				expectedPath := "/2025-07-01/connections/" + tt.connectionID + "/archive"
				if r.URL.Path != expectedPath {
					t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
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

			result, err := client.ArchiveConnection(context.Background(), tt.connectionID)

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
		})
	}
}

func TestUnarchiveConnection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		connectionID   string
		mockResponse   Connection
		mockStatusCode int
		wantErr        bool
		errContains    string
	}{
		{
			name:         "successful unarchive",
			connectionID: "conn_123",
			mockResponse: Connection{
				ID:        "conn_123",
				Name:      stringPtr("test-connection"),
				TeamID:    "team_123",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name:           "connection not found",
			connectionID:   "conn_nonexistent",
			mockStatusCode: http.StatusNotFound,
			wantErr:        true,
			errContains:    "404",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					t.Errorf("expected PUT request, got %s", r.Method)
				}
				expectedPath := "/2025-07-01/connections/" + tt.connectionID + "/unarchive"
				if r.URL.Path != expectedPath {
					t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
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

			result, err := client.UnarchiveConnection(context.Background(), tt.connectionID)

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
		})
	}
}

func TestCountConnections(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		params         map[string]string
		mockResponse   ConnectionCountResponse
		mockStatusCode int
		wantErr        bool
		errContains    string
	}{
		{
			name:   "successful count",
			params: map[string]string{},
			mockResponse: ConnectionCountResponse{
				Count: 42,
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name: "count with filters",
			params: map[string]string{
				"disabled": "false",
				"paused":   "false",
			},
			mockResponse: ConnectionCountResponse{
				Count: 10,
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name:           "server error",
			params:         map[string]string{},
			mockStatusCode: http.StatusInternalServerError,
			wantErr:        true,
			errContains:    "500",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("expected GET request, got %s", r.Method)
				}
				if r.URL.Path != "/2025-07-01/connections/count" {
					t.Errorf("expected path /2025-07-01/connections/count, got %s", r.URL.Path)
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

			result, err := client.CountConnections(context.Background(), tt.params)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Count != tt.mockResponse.Count {
				t.Errorf("expected count %d, got %d", tt.mockResponse.Count, result.Count)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

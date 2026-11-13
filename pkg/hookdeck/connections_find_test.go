package hookdeck

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

// TestFindConnectionByNameRequiresExactMatch pins that the name comparison is
// made here as well as by the API. The stub answers every query with the same
// list, standing in for an API whose name filter matched loosely.
func TestFindConnectionByNameRequiresExactMatch(t *testing.T) {
	t.Parallel()

	returned := []Connection{
		{ID: "web_partial", Name: stringPtr("orders-extra")},
		{ID: "web_case", Name: stringPtr("Orders")},
		{ID: "web_unnamed"},
		{ID: "web_exact", Name: stringPtr("orders")},
	}

	tests := []struct {
		name    string
		models  []Connection
		wantID  string
		wantNil bool
	}{
		{name: "exact match among loose ones is chosen", models: returned, wantID: "web_exact"},
		{name: "partial and case-different matches alone are not a match", models: returned[:3], wantNil: true},
		{name: "empty result", models: nil, wantNil: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Query().Get("name"); got != "orders" {
					t.Errorf("name filter = %q, want %q", got, "orders")
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(ConnectionListResponse{Models: tt.models})
			})
			defer server.Close()

			got, err := client.FindConnectionByName(context.Background(), "orders")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil {
				if got != nil {
					t.Fatalf("got %s, want no connection", got.ID)
				}
				return
			}
			if got == nil || got.ID != tt.wantID {
				t.Fatalf("got %+v, want %s", got, tt.wantID)
			}
		})
	}
}

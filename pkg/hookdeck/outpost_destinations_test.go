package hookdeck

import (
	"encoding/json"
	"testing"
)

// TestOutpostFilterPatch pins the distinction the update wire format depends on.
//
// Verified against the live API: PATCH {"filter":{}} clears a destination's
// filter, while a body with no filter key leaves it alone. On a plain map,
// omitempty collapsed both cases to "absent", so --filter '{}' reported success
// and changed nothing.
func TestOutpostFilterPatch(t *testing.T) {
	tests := []struct {
		name   string
		filter map[string]interface{}
		want   string
	}{
		{"not supplied", nil, `{}`},
		{"supplied as empty clears it", map[string]interface{}{}, `{"filter":{}}`},
		{"supplied with content", map[string]interface{}{"headers": "x"}, `{"filter":{"headers":"x"}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(&OutpostDestinationUpdateRequest{
				Filter: OutpostFilterPatch(tt.filter),
			})
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(body) != tt.want {
				t.Errorf("got %s, want %s", body, tt.want)
			}
		})
	}
}

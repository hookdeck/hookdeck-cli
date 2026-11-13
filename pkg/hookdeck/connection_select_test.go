package hookdeck

import "testing"

func connectionOn(id, name, sourceID, sourceName, destinationID string) Connection {
	return Connection{
		ID:          id,
		Name:        stringPtr(name),
		Source:      &Source{ID: sourceID, Name: sourceName},
		Destination: &Destination{ID: destinationID, Name: destinationID},
	}
}

func TestSelectConnectionForUpsert(t *testing.T) {
	t.Parallel()

	a := connectionOn("web_a", "orders", "src_a", "src-a", "des_a")
	b := connectionOn("web_b", "orders", "src_b", "src-b", "des_b")

	tests := []struct {
		name       string
		candidates []Connection
		sourceID   string
		sourceName string
		wantID     string
		wantSame   bool
		wantErr    bool
	}{
		{name: "no candidates", candidates: nil},
		{name: "one candidate and no source named", candidates: []Connection{a}, wantID: "web_a", wantSame: true},
		{name: "several candidates and no source named is ambiguous", candidates: []Connection{a, b}, wantErr: true},
		{name: "source named by ID picks the connection on it", candidates: []Connection{a, b}, sourceID: "src_b", wantID: "web_b", wantSame: true},
		{name: "source named by name picks the connection on it", candidates: []Connection{a, b}, sourceName: "src-a", wantID: "web_a", wantSame: true},
		{name: "a source no candidate uses is a different connection", candidates: []Connection{a}, sourceID: "src_other", wantID: "web_a", wantSame: false},
		{name: "a source name no candidate uses is a different connection", candidates: []Connection{a}, sourceName: "src-other", wantID: "web_a", wantSame: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, same, err := SelectConnectionForUpsert(tt.candidates, "orders", tt.sourceID, tt.sourceName)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantID == "" {
				if got != nil {
					t.Fatalf("got %s, want none", got.ID)
				}
				return
			}
			if got == nil || got.ID != tt.wantID || same != tt.wantSame {
				t.Fatalf("got %+v same=%v, want %s same=%v", got, same, tt.wantID, tt.wantSame)
			}
		})
	}
}

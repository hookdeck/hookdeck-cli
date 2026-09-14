package hookdeck

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestStatusVocabulariesMatchTheAPI pins the two enums against the OpenAPI
// document at https://api.hookdeck.com/2026-09-01/openapi.
//
// Expectations are hardcoded, so an API-side change will NOT fail this test;
// it catches the vocabularies drifting apart in here. They are easy to mix up
// because the same word means different things one route over.
func TestStatusVocabulariesMatchTheAPI(t *testing.T) {
	assert.Equal(t,
		[]string{"SCHEDULED", "QUEUED", "HOLD", "SUCCESSFUL", "FAILED", "CANCELLED"},
		EventStatusValueList,
		"the status enum of GET /events and GET /requests/{id}/events")
	assert.Equal(t,
		[]string{"accepted", "rejected"},
		RequestLogStatusValueList,
		"the status enum of GET /requests, which spells them lower case")
}

// TestEventStatusValuesRenderTheList stops the advertised vocabulary and the
// accepted one drifting: --help and the MCP schema read EventStatusValues,
// while the guards check EventStatusValueList.
func TestEventStatusValuesRenderTheList(t *testing.T) {
	assert.Equal(t, ValueList(EventStatusValueList), EventStatusValues)
	assert.Equal(t, ValueList(RequestLogStatusValueList), RequestLogStatusValues)
}

// TestCanonicalStatusValue covers the matching rule. Case is forgiven because
// the two routes disagree about it, but the canonical spelling is what gets
// sent: the API is case-sensitive and 422s "successful" against the upper-case
// enum.
func TestCanonicalStatusValue(t *testing.T) {
	tests := []struct {
		name       string
		vocabulary []string
		value      string
		want       string
		ok         bool
	}{
		{"exact", EventStatusValueList, "SUCCESSFUL", "SUCCESSFUL", true},
		{"lower case is canonicalised", EventStatusValueList, "successful", "SUCCESSFUL", true},
		{"mixed case is canonicalised", RequestLogStatusValueList, "Accepted", "accepted", true},
		{"other vocabulary is refused", EventStatusValueList, "accepted", "", false},
		{"unknown is refused", RequestLogStatusValueList, "bogus", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := CanonicalStatusValue(tt.vocabulary, tt.value)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

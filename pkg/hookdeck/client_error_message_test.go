package hookdeck

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAPIErrorMessageSurfacesTheUsefulLine covers the 422 bodies that were
// reaching callers whole.
//
// A validation failure carries no top-level "message": the one line worth
// reading sits in data[], so the entire body - internal fields included
// ({"level":"info","handled":true,"report":true,...}) - was pasted into the
// error text with the useful part buried in the middle of it. Every gated error
// in the metrics and requests tools eventually lands here.
func TestAPIErrorMessageSurfacesTheUsefulLine(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "422 validation body puts the message in data[]",
			body: `{"level":"info","handled":true,"report":true,"data":["The delivery_group dimension requires a filters.destination_id filter"],"status":422,"code":"UNPROCESSABLE_ENTITY"}`,
			want: "The delivery_group dimension requires a filters.destination_id filter",
		},
		{
			name: "measure enum rejection",
			body: `{"level":"info","handled":true,"data":["measures[0] must be one of [count, accepted_count]"],"status":422}`,
			want: "measures[0] must be one of [count, accepted_count]",
		},
		{
			name: "several data entries are joined",
			body: `{"data":["first problem","second problem"],"status":422}`,
			want: "first problem; second problem",
		},
		{
			name: "data entries may be objects",
			body: `{"data":[{"message":"nested problem"}],"status":422}`,
			want: "nested problem",
		},
		{
			name: "a top-level message still wins",
			body: `{"message":"Not found","data":["ignored"],"status":404}`,
			want: "Not found",
		},
		{
			name: "nothing readable falls back to the caller",
			body: `{"status":500}`,
			want: "",
		},
		{
			name: "a non-JSON body falls back to the caller",
			body: `bad gateway`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, apiErrorMessage([]byte(tt.body)))
		})
	}
}

// TestAPIErrorMessageDropsInternalFields is the point of the change, stated
// directly: whatever else happens, the transport's own bookkeeping must not
// reach an MCP client as error text.
func TestAPIErrorMessageDropsInternalFields(t *testing.T) {
	got := apiErrorMessage([]byte(`{"level":"info","handled":true,"report":true,"data":["dimensions[0] must be [destination_id]"],"status":422,"code":"UNPROCESSABLE_ENTITY"}`))
	for _, leaked := range []string{"level", "handled", "report", "UNPROCESSABLE_ENTITY"} {
		assert.NotContains(t, got, leaked)
	}
	assert.Equal(t, "dimensions[0] must be [destination_id]", got)
}

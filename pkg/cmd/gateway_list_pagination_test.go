package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The text output of these list commands tells the user to pass --next to get
// the next page, so each of them must accept --next/--prev and forward the
// cursor to the API.
func TestResourceListSendsPaginationCursors(t *testing.T) {
	for _, resource := range []string{"source", "destination", "connection"} {
		t.Run(resource, func(t *testing.T) {
			query := captureListQuery(t,
				"gateway", resource, "list",
				"--next", "cursor_next",
				"--prev", "cursor_prev",
			)
			assert.Equal(t, "cursor_next", query.Get("next"))
			assert.Equal(t, "cursor_prev", query.Get("prev"))
		})
	}
}

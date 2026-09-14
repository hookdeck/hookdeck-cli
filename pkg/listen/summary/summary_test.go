package summary

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestListeningAlwaysCarriesCounts pins #402: the summary must never degrade to
// a bare preposition. Both renderers build their banner from this string, so an
// empty or count-less result here is the compact-mode bug.
func TestListeningAlwaysCarriesCounts(t *testing.T) {
	tests := []struct {
		name        string
		sources     int
		connections int
		want        string
	}{
		{"singular", 1, 1, "Listening on 1 source • 1 connection"},
		{"plural", 2, 3, "Listening on 2 sources • 3 connections"},
		{"zero is still plural", 0, 0, "Listening on 0 sources • 0 connections"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Listening(tt.sources, tt.connections)
			assert.Equal(t, tt.want, got)
			assert.NotEqual(t, "Listening on", got, "a bare preposition is worse than nothing (#402)")
		})
	}
}

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestTranslateQueueDepthMeasures pins the mapping from the CLI's advertised
// "queue_depth" onto the measures the queue-depth endpoint actually accepts
// (max_depth, max_age).
func TestTranslateQueueDepthMeasures(t *testing.T) {
	assert.Equal(t, []string{"max_depth"}, translateQueueDepthMeasures([]string{"queue_depth"}))
	assert.Equal(t, []string{"max_depth"}, translateQueueDepthMeasures([]string{"max_depth"}))
	assert.Equal(t, []string{"max_age"}, translateQueueDepthMeasures([]string{"max_age"}))

	// Both spellings collapse to one measure rather than sending it twice.
	assert.Equal(t, []string{"max_depth"}, translateQueueDepthMeasures([]string{"queue_depth", "max_depth"}))
	assert.Equal(t, []string{"max_depth", "max_age"}, translateQueueDepthMeasures([]string{"queue_depth", "max_age"}))

	assert.Empty(t, translateQueueDepthMeasures(nil))
}

package cmd

import (
	"strings"
	"testing"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSourceCreateRequiresName asserts that create without --name fails (Cobra required-flag validation).
func TestSourceCreateRequiresName(t *testing.T) {
	rootCmd.SetArgs([]string{"gateway", "source", "create", "--type", "WEBHOOK"})
	err := rootCmd.Execute()
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "name") || strings.Contains(err.Error(), "required"),
		"error should mention name or required, got: %s", err.Error())
}

// TestSourceUpdateRequestEmpty asserts the "no updates specified" logic for update.
func TestSourceUpdateRequestEmpty(t *testing.T) {
	t.Run("empty request is empty", func(t *testing.T) {
		req := &hookdeck.SourceUpdateRequest{}
		assert.True(t, sourceUpdateRequestEmpty(req))
	})
	t.Run("name set is not empty", func(t *testing.T) {
		req := &hookdeck.SourceUpdateRequest{Name: "x"}
		assert.False(t, sourceUpdateRequestEmpty(req))
	})
	t.Run("config set is not empty", func(t *testing.T) {
		req := &hookdeck.SourceUpdateRequest{Config: map[string]interface{}{"webhook_secret": "x"}}
		assert.False(t, sourceUpdateRequestEmpty(req))
	})
	t.Run("type set is not empty", func(t *testing.T) {
		req := &hookdeck.SourceUpdateRequest{Type: "WEBHOOK"}
		assert.False(t, sourceUpdateRequestEmpty(req))
	})
	t.Run("description set is not empty", func(t *testing.T) {
		s := "desc"
		req := &hookdeck.SourceUpdateRequest{Description: &s}
		assert.False(t, sourceUpdateRequestEmpty(req))
	})
}

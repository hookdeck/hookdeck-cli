package cmd

import (
	"testing"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/stretchr/testify/assert"
)

// TestSourceCreateRequiresNameAndType verifies that the source create command
// marks --name and --type as required flags via cobra's MarkFlagRequired.
func TestSourceCreateRequiresNameAndType(t *testing.T) {
	sc := newSourceCreateCmd()

	nameFlag := sc.cmd.Flags().Lookup("name")
	assert.NotNil(t, nameFlag, "--name flag should exist")

	typeFlag := sc.cmd.Flags().Lookup("type")
	assert.NotNil(t, typeFlag, "--type flag should exist")

	// Cobra marks required flags with the "required" annotation
	nameAnnotations := nameFlag.Annotations
	assert.Contains(t, nameAnnotations, "cobra_annotation_bash_completion_one_required_flag",
		"--name should be marked as required")

	typeAnnotations := typeFlag.Annotations
	assert.Contains(t, typeAnnotations, "cobra_annotation_bash_completion_one_required_flag",
		"--type should be marked as required")
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

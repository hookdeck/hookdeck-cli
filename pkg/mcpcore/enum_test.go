package mcpcore

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A declared enum is a claim about what the tool accepts. Nothing checked it,
// so outpost_tenants_write minted a portal URL for theme "purple" while the
// same operation on the CLI refused it. checkArgumentTypes now enforces it,
// which is also what stops gateway_issues_write sending a status the API has
// never heard of.
func TestCheckArgumentTypes_Enum(t *testing.T) {
	props := map[string]Prop{
		actionArgName: {Type: "string", Enum: []string{"list", "portal"}},
		"theme":       {Type: "string", Enum: []string{"light", "dark"}},
		"note":        {Type: "string"},
		"limit":       {Type: "integer"},
	}

	t.Run("a value outside the enum is reported", func(t *testing.T) {
		problems := checkArgumentTypes(props, Input{"theme": "purple"})
		assert.Equal(t, []string{"theme must be one of: light, dark"}, problems)
	})

	t.Run("a declared value passes", func(t *testing.T) {
		assert.Empty(t, checkArgumentTypes(props, Input{"theme": "dark"}))
	})

	t.Run("a property with no enum is unaffected", func(t *testing.T) {
		assert.Empty(t, checkArgumentTypes(props, Input{"note": "anything at all"}))
	})

	t.Run("action is left to Dispatch", func(t *testing.T) {
		// Dispatch's message names the actions available in the current mode and
		// can redirect to the sibling tool; "must be one of" would replace it
		// with something strictly less useful.
		assert.Empty(t, checkArgumentTypes(props, Input{actionArgName: "nonesuch"}))
	})

	t.Run("the shape complaint wins over the enum", func(t *testing.T) {
		problems := checkArgumentTypes(props, Input{"theme": []interface{}{"light"}})
		assert.Equal(t, []string{"theme takes a single value, not an array"}, problems)
	})
}

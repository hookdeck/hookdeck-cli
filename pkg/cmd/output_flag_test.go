package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutputFormatValue(t *testing.T) {
	var out string
	v := newOutputFormatValue(&out)

	require.NoError(t, v.Set(""))
	require.NoError(t, v.Set("json"))
	assert.Equal(t, "json", out)

	err := v.Set("yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unsupported output format "yaml"`)
	assert.Contains(t, err.Error(), "supported: json")
	assert.Equal(t, "json", out, "a rejected value must not overwrite the previous one")
}

// TestJSONOutputFlagsRejectUnknownFormat checks every command that exposes a
// json-only --output flag, so a newly added command cannot silently ignore an
// unsupported format.
func TestJSONOutputFlagsRejectUnknownFormat(t *testing.T) {
	var checked int
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if f := c.Flags().Lookup("output"); f != nil {
			if _, ok := f.Value.(*outputFormatValue); ok {
				checked++
				assert.Error(t, c.Flags().Set("output", "yaml"),
					"command %q accepted --output yaml", c.CommandPath())
				assert.NoError(t, c.Flags().Set("output", "json"),
					"command %q rejected --output json", c.CommandPath())
			}
		}
		for _, child := range c.Commands() {
			walk(child)
		}
	}
	walk(RootCmd())
	assert.Greater(t, checked, 10, "expected many commands with a json-only --output flag")
}

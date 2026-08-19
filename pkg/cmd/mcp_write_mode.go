package cmd

import (
	"strconv"

	"github.com/spf13/cobra"
)

// allowWriteEnvVar enables write actions without a flag, for MCP clients whose
// config makes environment variables easier to set than arguments.
//
// It is shared by every MCP server the CLI starts: a user who wants write mode
// should not have to learn a different variable per product.
const allowWriteEnvVar = "HOOKDECK_MCP_ALLOW_WRITE"

// addWriteModeFlags registers the flags that select read-only or write mode.
func addWriteModeFlags(cmd *cobra.Command, allowWrite, readOnly *bool, allowWriteUsage string) {
	cmd.Flags().BoolVar(allowWrite, "allow-write", false, allowWriteUsage+" Also read from "+allowWriteEnvVar+"; the flag wins.")
	// Users arriving from other MCP servers type --read-only reflexively. It is
	// already the default, so accept it rather than failing on an unknown flag.
	cmd.Flags().BoolVar(readOnly, "read-only", false, "Run without write actions. This is the default; the flag is accepted so it can be passed explicitly, and wins over --allow-write.")
}

// resolveAllowWrite decides whether write actions are enabled.
//
// --read-only wins over everything so an explicit request for a safe session is
// never overridden; otherwise --allow-write wins over the environment variable,
// which is the more distant and easier-to-forget setting.
func resolveAllowWrite(allowWriteFlag, allowWriteFlagSet, readOnly bool, envValue string) bool {
	if readOnly {
		return false
	}
	if allowWriteFlagSet {
		return allowWriteFlag
	}
	enabled, err := strconv.ParseBool(envValue)
	if err != nil {
		return false
	}
	return enabled
}

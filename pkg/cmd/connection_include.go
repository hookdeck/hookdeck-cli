package cmd

import "github.com/spf13/cobra"

// addIncludeDestinationAuthFlag registers the --include-destination-auth flag on a cobra command.
// When set, the CLI fetches destination auth credentials via
// GET /destinations/{id}?include=config.auth and merges them into the response.
func addIncludeDestinationAuthFlag(cmd *cobra.Command, target *bool) {
	cmd.Flags().BoolVar(target, "include-destination-auth", false,
		"Include destination authentication credentials in the response")
}

// includeAuthParams returns a map with the include query parameter set
// if includeAuth is true, or nil otherwise.
func includeAuthParams(includeAuth bool) map[string]string {
	if includeAuth {
		return map[string]string{"include": "config.auth"}
	}
	return nil
}

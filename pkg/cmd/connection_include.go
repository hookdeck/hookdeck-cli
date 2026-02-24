package cmd

import "github.com/spf13/cobra"

// addIncludeAuthFlagForDestination registers the --include-auth flag on a destination get command.
// When set, the CLI requests destination auth via GET /destinations/{id}?include=config.auth.
func addIncludeAuthFlagForDestination(cmd *cobra.Command, target *bool) {
	cmd.Flags().BoolVar(target, "include-auth", false,
		"Include authentication credentials in the response")
}

// addIncludeSourceAuthFlagForConnection registers the --include-source-auth flag on a connection get command.
func addIncludeSourceAuthFlagForConnection(cmd *cobra.Command, target *bool) {
	cmd.Flags().BoolVar(target, "include-source-auth", false,
		"Include source authentication credentials in the response")
}

// addIncludeDestinationAuthFlag registers the --include-destination-auth flag on a connection get command.
// Use the fully qualified name on connection since connection get can include source or destination auth.
func addIncludeDestinationAuthFlag(cmd *cobra.Command, target *bool) {
	cmd.Flags().BoolVar(target, "include-destination-auth", false,
		"Include destination authentication credentials in the response")
}

// addIncludeSourceAuthFlag registers the --include-auth flag on a cobra command (e.g. source get).
// When set, the CLI requests source auth via GET /sources/{id}?include=config.auth.
func addIncludeSourceAuthFlag(cmd *cobra.Command, target *bool) {
	cmd.Flags().BoolVar(target, "include-auth", false,
		"Include source authentication credentials in the response")
}

// includeAuthParams returns a map with the include query parameter set
// if includeAuth is true, or nil otherwise.
func includeAuthParams(includeAuth bool) map[string]string {
	if includeAuth {
		return map[string]string{"include": "config.auth"}
	}
	return nil
}

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

// org is the platform group: the organization this credential belongs to, and
// the API keys issued within it.
//
// There is no organization argument anywhere under it, because the API offers
// none — every route is /organizations/current, resolved from the credential in
// use. To act on a different organization, sign in with one of its credentials.
type orgCmd struct {
	cmd    *cobra.Command
	apiKey string
}

func newOrgCmd() *orgCmd {
	oc := &orgCmd{}

	oc.cmd = &cobra.Command{
		Use:     "org",
		Aliases: []string{"organization"},
		Args:    validators.NoArgs,
		Short:   "Manage your organization [BETA]",
		Long: `Read or rename the Hookdeck organization your credentials belong to, and manage its API keys.

These commands need an ORGANIZATION API key, which is a different credential from the one
` + "`hookdeck login`" + ` stores. A CLI session can list projects and work inside one, but cannot read
the organization — the API answers a bare 401 that reads as a bad key rather than the wrong kind.

Create an organization API key in the Hookdeck dashboard, then pass it with --api-key or set
HOOKDECK_API_KEY. Note that ` + "`hookdeck ci --api-key`" + ` will not take one: that path wants a
project key.

The API acts on the current organization only, determined by the credential in use.
There is no way to name another one; sign in with one of its credentials instead.`,
	}

	// Organization API keys are a distinct credential class, and the CLI has no
	// other path to one: `hookdeck login --api-key` is the CLI-key path and
	// `hookdeck ci --api-key` wants a project key. Both refuse an organization
	// key. Same shape as `outpost publish`, which carries its own --api-key
	// because the publish API will not take a CLI session either.
	oc.cmd.PersistentFlags().StringVar(&oc.apiKey, "api-key", "",
		"Hookdeck organization API key. Read from HOOKDECK_API_KEY when not provided.")
	oc.cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// Every command carrying a PersistentPreRun has to do this, or its
		// invocations go unrecorded — pinned by
		// TestAllCommandsWithPersistentPreRunInitTelemetry.
		initTelemetry(cmd)

		key := oc.apiKey
		if key == "" {
			key = envAPIKey()
		}
		if key != "" {
			Config.Profile.APIKey = key
		}
		return nil
	}

	oc.cmd.AddCommand(newOrgGetCmd().cmd)
	oc.cmd.AddCommand(newOrgUpdateCmd().cmd)
	oc.cmd.AddCommand(newAPIKeyCmd().cmd)

	return oc
}

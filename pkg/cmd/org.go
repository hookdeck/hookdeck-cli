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
	cmd *cobra.Command
}

func newOrgCmd() *orgCmd {
	oc := &orgCmd{}

	oc.cmd = &cobra.Command{
		Use:     "org",
		Aliases: []string{"organization"},
		Args:    validators.NoArgs,
		Short:   "Manage your organization [BETA]",
		Long: `Read or rename the Hookdeck organization your credentials belong to, and manage its API keys.

The API acts on the current organization only, determined by the credential in use.
There is no way to name another one; sign in with one of its credentials instead.`,
	}

	oc.cmd.AddCommand(newOrgGetCmd().cmd)
	oc.cmd.AddCommand(newOrgUpdateCmd().cmd)
	oc.cmd.AddCommand(newAPIKeyCmd().cmd)

	return oc
}

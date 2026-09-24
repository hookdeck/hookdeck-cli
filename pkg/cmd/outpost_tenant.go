package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type outpostTenantCmd struct {
	cmd *cobra.Command
}

func newOutpostTenantCmd() *outpostTenantCmd {
	tc := &outpostTenantCmd{}

	tc.cmd = &cobra.Command{
		Use:     "tenant",
		Aliases: []string{"tenants"},
		Args:    validators.NoArgs,
		Short:   ShortBeta("Manage your Outpost tenants"),
		Long: LongBeta(`Manage tenants — the end users events are delivered on behalf of.

Each tenant owns its own destinations. Tenant IDs are chosen by you rather than
generated, so use 'upsert' to create one: it is idempotent and safe to re-run.`),
	}

	tc.cmd.AddCommand(newOutpostTenantListCmd().cmd)
	tc.cmd.AddCommand(newOutpostTenantGetCmd().cmd)
	tc.cmd.AddCommand(newOutpostTenantUpsertCmd().cmd)
	tc.cmd.AddCommand(newOutpostTenantDeleteCmd().cmd)
	tc.cmd.AddCommand(newOutpostTenantTokenCmd().cmd)
	tc.cmd.AddCommand(newOutpostTenantPortalCmd().cmd)

	return tc
}

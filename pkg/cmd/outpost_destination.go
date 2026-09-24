package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type outpostDestinationCmd struct {
	cmd *cobra.Command

	// tenantID is persistent across the group: every destination endpoint is
	// scoped to a tenant, so requiring it once per subcommand would be noise.
	tenantID string
}

func newOutpostDestinationCmd() *outpostDestinationCmd {
	dc := &outpostDestinationCmd{}

	dc.cmd = &cobra.Command{
		Use:     "destination",
		Aliases: []string{"destinations"},
		Args:    validators.NoArgs,
		Short:   ShortBeta("Manage your Outpost destinations"),
		Long: LongBeta(`Manage the destinations events are delivered to.

Destinations belong to a tenant, so every command here takes --tenant-id.

Config and credential fields depend on the destination type. Pass them as
repeatable key=value pairs — for example '--config url=https://example.com' — and
run 'hookdeck outpost destination-type get <type>' to see what a type accepts.`),
	}

	dc.cmd.PersistentFlags().StringVar(&dc.tenantID, "tenant-id", "", "The tenant that owns the destination (required)")

	dc.cmd.AddCommand(newOutpostDestinationListCmd(dc).cmd)
	dc.cmd.AddCommand(newOutpostDestinationGetCmd(dc).cmd)
	dc.cmd.AddCommand(newOutpostDestinationCreateCmd(dc).cmd)
	dc.cmd.AddCommand(newOutpostDestinationUpdateCmd(dc).cmd)
	dc.cmd.AddCommand(newOutpostDestinationDeleteCmd(dc).cmd)
	dc.cmd.AddCommand(newOutpostDestinationEnableCmd(dc).cmd)
	dc.cmd.AddCommand(newOutpostDestinationDisableCmd(dc).cmd)

	return dc
}

// requireTenantID reports a missing --tenant-id in the same terms as the flag,
// rather than letting the request go out and fail as a 404.
func (dc *outpostDestinationCmd) requireTenantID() error {
	if dc.tenantID == "" {
		return errMissingTenantID
	}
	return nil
}

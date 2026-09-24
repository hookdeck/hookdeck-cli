package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type outpostTenantDeleteCmd struct {
	cmd *cobra.Command

	force bool
}

func newOutpostTenantDeleteCmd() *outpostTenantDeleteCmd {
	tc := &outpostTenantDeleteCmd{}

	tc.cmd = &cobra.Command{
		Use:   "delete <tenant-id>",
		Args:  validators.ExactArgs(1),
		Short: ShortDelete(ResourceTenant),
		Long: LongDeleteIntro(ResourceTenant) + `

Deleting a tenant also removes its destinations, so events will stop being
delivered on its behalf. This cannot be undone.`,
		RunE: tc.runOutpostTenantDeleteCmd,
		Example: `  # Delete a tenant, with a confirmation prompt
  hookdeck outpost tenant delete acme

  # Skip the prompt (for scripts and CI)
  hookdeck outpost tenant delete acme --force`,
		Annotations: map[string]string{
			"cli.arguments": `[
				{"name":"tenant-id","type":"string","description":"The ID of the tenant to delete.","required":true}
			]`,
		},
	}

	tc.cmd.Flags().BoolVar(&tc.force, "force", false, "Delete without confirmation")

	return tc
}

func (tc *outpostTenantDeleteCmd) runOutpostTenantDeleteCmd(cmd *cobra.Command, args []string) error {
	tenantID := args[0]
	client := Config.GetOutpostAPIClient()
	ctx := context.Background()

	if !tc.force {
		// Report the blast radius rather than just the name: the destination
		// count is the part a user is most likely to have forgotten.
		prompt := fmt.Sprintf("\nAre you sure you want to delete tenant '%s'?", tenantID)
		if tenant, err := client.GetOutpostTenant(ctx, tenantID); err == nil && tenant.DestinationsCount > 0 {
			prompt = fmt.Sprintf(
				"\nAre you sure you want to delete tenant '%s' and its %d destination(s)?",
				tenantID, tenant.DestinationsCount,
			)
		}

		proceed, err := confirmDestructiveAction(prompt, "Deletion cancelled.", "force")
		if err != nil {
			return err
		}
		if !proceed {
			return nil
		}
	}

	if err := client.DeleteOutpostTenant(ctx, tenantID); err != nil {
		return fmt.Errorf("failed to delete tenant: %w", err)
	}

	fmt.Printf("%s Tenant %s deleted\n", SuccessCheck, tenantID)

	return nil
}

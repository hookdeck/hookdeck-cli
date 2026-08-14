package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type outpostTenantGetCmd struct {
	cmd *cobra.Command

	output string
}

func newOutpostTenantGetCmd() *outpostTenantGetCmd {
	tc := &outpostTenantGetCmd{}

	tc.cmd = &cobra.Command{
		Use:   "get <tenant-id>",
		Args:  validators.ExactArgs(1),
		Short: ShortGet(ResourceTenant),
		Long:  `Get details for a tenant, including how many destinations it has.`,
		RunE:  tc.runOutpostTenantGetCmd,
		Example: `  # Get a tenant
  hookdeck outpost tenant get acme

  # As JSON
  hookdeck outpost tenant get acme --output json`,
		Annotations: map[string]string{
			"cli.arguments": `[
				{"name":"tenant-id","type":"string","description":"The ID of the tenant.","required":true}
			]`,
		},
	}

	tc.cmd.Flags().StringVar(&tc.output, "output", "", "Output format (json)")

	return tc
}

func (tc *outpostTenantGetCmd) runOutpostTenantGetCmd(cmd *cobra.Command, args []string) error {
	client := Config.GetOutpostAPIClient()

	tenant, err := client.GetOutpostTenant(context.Background(), args[0])
	if err != nil {
		return fmt.Errorf("failed to get tenant: %w", err)
	}

	if tc.output == "json" {
		return printJSONIndented(tenant)
	}

	color := ansi.Color(os.Stdout)
	fmt.Printf("\n%s\n", color.Green(tenant.ID))
	fmt.Printf("  Destinations: %d\n", tenant.DestinationsCount)
	if len(tenant.Topics) > 0 {
		fmt.Printf("  Topics: %s\n", strings.Join(tenant.Topics, ", "))
	}
	for key, value := range tenant.Metadata {
		fmt.Printf("  Metadata %s: %s\n", key, value)
	}
	fmt.Printf("  Created: %s\n", tenant.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("  Updated: %s\n", tenant.UpdatedAt.Format("2006-01-02 15:04:05"))
	fmt.Println()

	return nil
}

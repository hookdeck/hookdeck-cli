package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type outpostTenantUpsertCmd struct {
	cmd *cobra.Command

	metadata     []string
	metadataFile string
	output       string
}

func newOutpostTenantUpsertCmd() *outpostTenantUpsertCmd {
	tc := &outpostTenantUpsertCmd{}

	tc.cmd = &cobra.Command{
		Use:   "upsert <tenant-id>",
		Args:  validators.ExactArgs(1),
		Short: ShortUpsert(ResourceTenant),
		Long: LongUpsertIntro(ResourceTenant) + `

Tenant IDs are chosen by you, not generated, so this is the only way to create one.
Re-running with the same ID updates the tenant's metadata rather than failing.

Metadata is replaced wholesale, not merged: pass every key you want to keep.
Supplying no metadata at all clears it, so an upsert run only to make sure a
tenant exists will remove metadata it already had.`,
		PreRunE: tc.validateFlags,
		RunE:    tc.runOutpostTenantUpsertCmd,
		Example: `  # Create a tenant, or clear the metadata of one that exists
  hookdeck outpost tenant upsert acme

  # With metadata
  hookdeck outpost tenant upsert acme --metadata plan=pro --metadata region=eu

  # Metadata from a JSON file
  hookdeck outpost tenant upsert acme --metadata-file ./tenant.json`,
		Annotations: map[string]string{
			"cli.arguments": `[
				{"name":"tenant-id","type":"string","description":"The ID of the tenant to create or update.","required":true}
			]`,
		},
	}

	tc.cmd.Flags().StringArrayVar(&tc.metadata, "metadata", nil, "Metadata as key=value (repeatable)")
	tc.cmd.Flags().StringVar(&tc.metadataFile, "metadata-file", "", "Path to a JSON file of metadata key/value pairs")
	tc.cmd.Flags().StringVar(&tc.output, "output", "", "Output format (json)")

	return tc
}

func (tc *outpostTenantUpsertCmd) validateFlags(cmd *cobra.Command, args []string) error {
	if err := rejectEmptyFlags(cmd); err != nil {
		return err
	}
	if len(tc.metadata) > 0 && tc.metadataFile != "" {
		return fmt.Errorf("--metadata and --metadata-file cannot be used together")
	}
	return nil
}

func (tc *outpostTenantUpsertCmd) runOutpostTenantUpsertCmd(cmd *cobra.Command, args []string) error {
	metadata, err := tc.resolveMetadata()
	if err != nil {
		return err
	}

	client := Config.GetOutpostAPIClient()

	tenant, err := client.UpsertOutpostTenant(context.Background(), args[0], &hookdeck.OutpostTenantUpsertRequest{
		Metadata: metadata,
	})
	if err != nil {
		return fmt.Errorf("failed to upsert tenant: %w", err)
	}

	if tc.output == "json" {
		return printJSONIndented(tenant)
	}

	fmt.Printf("%s Tenant %s saved\n", SuccessCheck, tenant.ID)

	return nil
}

func (tc *outpostTenantUpsertCmd) resolveMetadata() (map[string]string, error) {
	return resolveOutpostMetadata(tc.metadata, tc.metadataFile)
}

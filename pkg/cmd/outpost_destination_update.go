package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type outpostDestinationUpdateCmd struct {
	cmd    *cobra.Command
	parent *outpostDestinationCmd

	fields outpostDestinationFieldFlags
	output string
}

func newOutpostDestinationUpdateCmd(parent *outpostDestinationCmd) *outpostDestinationUpdateCmd {
	dc := &outpostDestinationUpdateCmd{parent: parent}

	dc.cmd = &cobra.Command{
		Use:   "update <destination-id>",
		Args:  validators.ExactArgs(1),
		Short: ShortUpdate(ResourceDestination),
		Long: LongUpdateIntro(ResourceDestination) + `

Only the fields you pass are changed; omitted fields are left alone.

--filter and --metadata are the exceptions: the API replaces each wholesale
rather than merging into it, so pass the complete value you want.`,
		PreRunE: dc.validateFlags,
		RunE:    dc.runOutpostDestinationUpdateCmd,
		Example: `  # Point a destination at a new URL
  hookdeck outpost destination update des_abc123 --tenant-id acme \
    --config url=https://example.com/new

  # Change which topics it receives
  hookdeck outpost destination update des_abc123 --tenant-id acme --topics "*"

  # Replace the metadata
  hookdeck outpost destination update des_abc123 --tenant-id acme \
    --metadata owner=platform --metadata tier=pro`,
		Annotations: map[string]string{
			"cli.arguments": `[
				{"name":"destination-id","type":"string","description":"The ID of the destination to update.","required":true}
			]`,
		},
	}

	dc.cmd.Flags().StringVar(&dc.output, "output", "", "Output format (json)")
	addOutpostDestinationFieldFlags(dc.cmd, &dc.fields)

	return dc
}

func (dc *outpostDestinationUpdateCmd) validateFlags(cmd *cobra.Command, args []string) error {
	if err := rejectEmptyFlags(cmd); err != nil {
		return err
	}
	if err := dc.parent.requireTenantID(); err != nil {
		return err
	}
	if err := dc.fields.validate(); err != nil {
		return err
	}
	// An update with nothing to update is a no-op that looks like a success.
	if !dc.fields.hasAny() {
		return fmt.Errorf("nothing to update. Pass at least one of --config, --credential, --topics, --filter or --metadata")
	}
	return nil
}

func (dc *outpostDestinationUpdateCmd) runOutpostDestinationUpdateCmd(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	config, err := dc.fields.resolveConfig()
	if err != nil {
		return err
	}
	credentials, err := dc.fields.resolveCredentials()
	if err != nil {
		return err
	}
	filter, err := dc.fields.resolveFilter()
	if err != nil {
		return err
	}
	metadata, err := dc.fields.resolveMetadata()
	if err != nil {
		return err
	}

	client := Config.GetOutpostAPIClient()

	// The type is fixed at creation, so it is read back to validate the fields
	// being changed rather than asking the user to repeat it.
	//
	// Only the supplied keys are checked. The endpoint is a PATCH that leaves
	// omitted fields alone, so enforcing the type's required fields here
	// rejected valid partial updates — rotating a credential failed with
	// "--config url=<value> is required" before any request was sent.
	if len(config) > 0 || len(credentials) > 0 {
		existing, err := client.GetOutpostDestination(ctx, dc.parent.tenantID, args[0])
		if err != nil {
			return fmt.Errorf("failed to look up destination: %w", err)
		}
		if err := validateOutpostDestinationFields(ctx, existing.Type, config, credentials, validateForUpdate); err != nil {
			return err
		}
	}

	destination, err := client.UpdateOutpostDestination(ctx, dc.parent.tenantID, args[0], &hookdeck.OutpostDestinationUpdateRequest{
		Topics:      dc.fields.resolveTopics(),
		Config:      config,
		Credentials: credentials,
		Filter:      hookdeck.OutpostFilterPatch(filter),
		Metadata:    metadata,
	})
	if err != nil {
		return fmt.Errorf("failed to update destination: %w", err)
	}

	if dc.output == "json" {
		return printJSONIndented(destination)
	}

	fmt.Printf("%s Destination updated\n\n", SuccessCheck)
	printOutpostDestination(destination, "")
	fmt.Println()

	return nil
}

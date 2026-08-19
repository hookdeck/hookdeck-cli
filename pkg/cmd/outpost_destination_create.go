package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type outpostDestinationCreateCmd struct {
	cmd    *cobra.Command
	parent *outpostDestinationCmd

	fields   outpostDestinationFieldFlags
	destType string
	output   string
}

func newOutpostDestinationCreateCmd(parent *outpostDestinationCmd) *outpostDestinationCreateCmd {
	dc := &outpostDestinationCreateCmd{parent: parent}

	dc.cmd = &cobra.Command{
		Use:   "create",
		Args:  validators.NoArgs,
		Short: ShortCreate(ResourceDestination),
		Long: `Create a destination for a tenant.

Config and credential fields depend on --type. Pass them as repeatable key=value
pairs; run 'hookdeck outpost destination-type list' to see the available types and
'hookdeck outpost destination-type get <type>' to see the fields one accepts.

Topics default to all ("*") when --topics is omitted.`,
		PreRunE: dc.validateFlags,
		RunE:    dc.runOutpostDestinationCreateCmd,
		Example: `  # A webhook destination subscribed to everything
  hookdeck outpost destination create --tenant-id acme --type webhook \
    --config url=https://example.com/hooks

  # Subscribed to specific topics
  hookdeck outpost destination create --tenant-id acme --type webhook \
    --config url=https://example.com/hooks --topics user.created,user.updated

  # With credentials and a filter
  hookdeck outpost destination create --tenant-id acme --type aws_sqs \
    --config queue_url=https://sqs.eu-west-2.amazonaws.com/1/q \
    --credential key=AKIA... --credential secret=... \
    --filter '{"data":{"tier":"pro"}}'

  # With metadata of your own to correlate against your systems
  hookdeck outpost destination create --tenant-id acme --type webhook \
    --config url=https://example.com/hooks \
    --metadata owner=platform --metadata tier=pro`,
	}

	dc.cmd.Flags().StringVar(&dc.destType, "type", "", "Destination type (required)")
	dc.cmd.Flags().StringVar(&dc.output, "output", "", "Output format (json)")
	addOutpostDestinationFieldFlags(dc.cmd, &dc.fields)
	dc.cmd.MarkFlagRequired("type")

	// `--type X --help` lists that type's fields; plain `--help` is untouched.
	addOutpostDestinationTypeHelp(dc.cmd, &dc.destType)

	return dc
}

func (dc *outpostDestinationCreateCmd) validateFlags(cmd *cobra.Command, args []string) error {
	if err := rejectEmptyFlags(cmd); err != nil {
		return err
	}
	if err := dc.parent.requireTenantID(); err != nil {
		return err
	}
	return dc.fields.validate()
}

func (dc *outpostDestinationCreateCmd) runOutpostDestinationCreateCmd(cmd *cobra.Command, args []string) error {
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

	if err := validateOutpostDestinationFields(ctx, dc.destType, config, credentials); err != nil {
		return err
	}

	topics := dc.fields.resolveTopics()
	if topics == nil {
		// The API requires topics, so default to everything rather than failing
		// on an omitted flag.
		topics = hookdeck.OutpostTopics{hookdeck.OutpostTopicsWildcard}
	}

	client := Config.GetOutpostAPIClient()

	destination, err := client.CreateOutpostDestination(ctx, dc.parent.tenantID, &hookdeck.OutpostDestinationCreateRequest{
		Type:        dc.destType,
		Topics:      topics,
		Config:      config,
		Credentials: credentials,
		Filter:      filter,
		Metadata:    metadata,
	})
	if err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}

	if dc.output == "json" {
		return printJSONIndented(destination)
	}

	fmt.Printf("%s Destination created\n\n", SuccessCheck)
	printOutpostDestination(destination, "")
	fmt.Println()

	return nil
}

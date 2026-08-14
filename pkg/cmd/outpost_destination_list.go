package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

// errMissingTenantID is shared by every destination subcommand so the guidance
// stays identical wherever it surfaces.
var errMissingTenantID = errors.New("--tenant-id is required. Run 'hookdeck outpost tenant list' to see available tenants")

type outpostDestinationListCmd struct {
	cmd    *cobra.Command
	parent *outpostDestinationCmd

	destType string
	topics   string
	output   string
}

func newOutpostDestinationListCmd(parent *outpostDestinationCmd) *outpostDestinationListCmd {
	dc := &outpostDestinationListCmd{parent: parent}

	dc.cmd = &cobra.Command{
		Use:   "list",
		Args:  validators.NoArgs,
		Short: ShortList(ResourceDestination),
		Long: `List a tenant's destinations.

This endpoint is not paginated: every destination for the tenant is returned.`,
		PreRunE: dc.validateFlags,
		RunE:    dc.runOutpostDestinationListCmd,
		Example: `  # List a tenant's destinations
  hookdeck outpost destination list --tenant-id acme

  # Filter by type or topic
  hookdeck outpost destination list --tenant-id acme --type webhook
  hookdeck outpost destination list --tenant-id acme --topics user.created`,
	}

	dc.cmd.Flags().StringVar(&dc.destType, "type", "", "Filter by destination type(s), comma-separated")
	dc.cmd.Flags().StringVar(&dc.topics, "topics", "", "Filter by topic(s), comma-separated")
	dc.cmd.Flags().StringVar(&dc.output, "output", "", "Output format (json)")

	return dc
}

func (dc *outpostDestinationListCmd) validateFlags(cmd *cobra.Command, args []string) error {
	if err := rejectEmptyFlags(cmd); err != nil {
		return err
	}
	return dc.parent.requireTenantID()
}

func (dc *outpostDestinationListCmd) runOutpostDestinationListCmd(cmd *cobra.Command, args []string) error {
	client := Config.GetOutpostAPIClient()

	destinations, err := client.ListOutpostDestinations(
		context.Background(),
		dc.parent.tenantID,
		splitCommaList(dc.destType),
		splitCommaList(dc.topics),
	)
	if err != nil {
		return fmt.Errorf("failed to list destinations: %w", err)
	}

	if dc.output == "json" {
		return printJSONIndented(destinations)
	}

	if len(destinations) == 0 {
		fmt.Println("No destinations found.")
		return nil
	}

	fmt.Printf("\nFound %d destination(s) for tenant %s:\n\n", len(destinations), dc.parent.tenantID)
	for i := range destinations {
		printOutpostDestination(&destinations[i], "")
		fmt.Println()
	}

	return nil
}

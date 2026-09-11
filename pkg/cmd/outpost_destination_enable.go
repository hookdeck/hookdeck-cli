package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type outpostDestinationEnableCmd struct {
	cmd    *cobra.Command
	parent *outpostDestinationCmd

	output string
}

func newOutpostDestinationEnableCmd(parent *outpostDestinationCmd) *outpostDestinationEnableCmd {
	dc := &outpostDestinationEnableCmd{parent: parent}

	dc.cmd = &cobra.Command{
		Use:     "enable <destination-id>",
		Args:    validators.ExactArgs(1),
		Short:   ShortEnable(ResourceDestination),
		Long:    LongEnableIntro(ResourceDestination),
		PreRunE: dc.validateFlags,
		RunE:    dc.runOutpostDestinationEnableCmd,
		Example: `  # Resume delivery to a destination
  hookdeck outpost destination enable des_abc123 --tenant-id acme`,
		Annotations: map[string]string{
			"cli.arguments": `[
				{"name":"destination-id","type":"string","description":"The ID of the destination to enable.","required":true}
			]`,
		},
	}

	dc.cmd.Flags().StringVar(&dc.output, "output", "", "Output format (json)")

	return dc
}

func (dc *outpostDestinationEnableCmd) validateFlags(cmd *cobra.Command, args []string) error {
	if err := rejectEmptyFlags(cmd); err != nil {
		return err
	}
	return dc.parent.requireTenantID()
}

func (dc *outpostDestinationEnableCmd) runOutpostDestinationEnableCmd(cmd *cobra.Command, args []string) error {
	client := Config.GetOutpostAPIClient()

	destination, err := client.EnableOutpostDestination(context.Background(), dc.parent.tenantID, args[0])
	if err != nil {
		return fmt.Errorf("failed to enable destination: %w", err)
	}

	return printOutpostDestinationStateChange(destination, dc.output, "enabled")
}

// printOutpostDestinationStateChange is shared by enable and disable, which
// differ only in the verb they intend.
//
// It reports the state the API returned rather than the verb that was asked
// for. Announcing "Destination X enabled" after a call that left it disabled is
// the same wrong-answer-that-reads-as-right the gateway event commands had:
// message and exit code both claimed success while nothing had changed.
func printOutpostDestinationStateChange(destination *hookdeck.OutpostDestination, output, verb string) error {
	actual := "enabled"
	if destination.Disabled() {
		actual = "disabled"
	}

	if output == "json" {
		// Print the payload either way — it is the evidence — then fail if the
		// state does not match what was asked for.
		if err := printJSONIndented(destination); err != nil {
			return err
		}
		if actual != verb {
			return fmt.Errorf("destination %s is %s, not %s", destination.ID, actual, verb)
		}
		return nil
	}

	if actual != verb {
		return fmt.Errorf("destination %s is %s, not %s", destination.ID, actual, verb)
	}
	fmt.Printf("%s Destination %s %s\n", SuccessCheck, destination.ID, actual)
	return nil
}

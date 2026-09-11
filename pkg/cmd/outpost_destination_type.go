package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/cmd/outposttypes"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type outpostDestinationTypeCmd struct {
	cmd *cobra.Command
}

func newOutpostDestinationTypeCmd() *outpostDestinationTypeCmd {
	dc := &outpostDestinationTypeCmd{}

	dc.cmd = &cobra.Command{
		Use:     "destination-type",
		Aliases: []string{"destination-types"},
		Args:    validators.NoArgs,
		Short:   ShortBeta("Inspect available destination types"),
		Long: LongBeta(`Inspect the destination types this project can create, and the fields each accepts.

Destination types are defined by the Outpost deployment rather than the CLI, so
this is the authoritative list — it stays correct as new types are added.`),
	}

	dc.cmd.AddCommand(newOutpostDestinationTypeListCmd().cmd)
	dc.cmd.AddCommand(newOutpostDestinationTypeGetCmd().cmd)

	return dc
}

type outpostDestinationTypeListCmd struct {
	cmd *cobra.Command

	output string
}

func newOutpostDestinationTypeListCmd() *outpostDestinationTypeListCmd {
	dc := &outpostDestinationTypeListCmd{}

	dc.cmd = &cobra.Command{
		Use:   "list",
		Args:  validators.NoArgs,
		Short: ShortList(ResourceDestinationType),
		Long:  `List the destination types available in this project.`,
		RunE:  dc.run,
		Example: `  # List available destination types
  hookdeck outpost destination-type list`,
	}

	dc.cmd.Flags().StringVar(&dc.output, "output", "", "Output format (json)")

	return dc
}

func (dc *outpostDestinationTypeListCmd) run(cmd *cobra.Command, args []string) error {
	client := Config.GetOutpostAPIClient()

	schemas, err := outposttypes.FetchDestinationTypes(context.Background(), client)
	if err != nil {
		return fmt.Errorf("failed to list destination types: %w", err)
	}

	if dc.output == "json" {
		return printJSONIndented(schemas)
	}

	color := ansi.Color(os.Stdout)
	fmt.Printf("\nFound %d destination type(s):\n\n", len(schemas))
	for _, schema := range schemas {
		fmt.Printf("%s\n", color.Green(schema.Type))
		if schema.Label != "" {
			fmt.Printf("  %s\n", schema.Label)
		}
		if schema.Description != "" {
			fmt.Printf("  %s\n", schema.Description)
		}
		fmt.Println()
	}
	fmt.Println("Run 'hookdeck outpost destination-type get <type>' to see the fields a type accepts.")

	return nil
}

type outpostDestinationTypeGetCmd struct {
	cmd *cobra.Command

	output string
}

func newOutpostDestinationTypeGetCmd() *outpostDestinationTypeGetCmd {
	dc := &outpostDestinationTypeGetCmd{}

	dc.cmd = &cobra.Command{
		Use:   "get <type>",
		Args:  validators.ExactArgs(1),
		Short: ShortGet(ResourceDestinationType),
		Long: `Show the config and credential fields a destination type accepts.

Each field lists whether it is required, whether it is sensitive, and any values
or format the schema constrains it to.`,
		RunE: dc.run,
		Example: `  # Show the fields a webhook destination accepts
  hookdeck outpost destination-type get webhook`,
		Annotations: map[string]string{
			"cli.arguments": `[
				{"name":"type","type":"string","description":"The destination type to describe (e.g. webhook, aws_sqs).","required":true}
			]`,
		},
	}

	dc.cmd.Flags().StringVar(&dc.output, "output", "", "Output format (json)")

	return dc
}

func (dc *outpostDestinationTypeGetCmd) run(cmd *cobra.Command, args []string) error {
	client := Config.GetOutpostAPIClient()
	ctx := context.Background()

	schemas, err := outposttypes.FetchDestinationTypes(ctx, client)
	if err != nil {
		return fmt.Errorf("failed to fetch destination types: %w", err)
	}

	schema, found := outposttypes.Find(schemas, args[0])
	if !found {
		return fmt.Errorf("unknown destination type %q. Available types: %s",
			args[0], strings.Join(outposttypes.TypeNames(schemas), ", "))
	}

	if dc.output == "json" {
		return printJSONIndented(schema)
	}

	color := ansi.Color(os.Stdout)
	fmt.Printf("\n%s\n", color.Green(schema.Type))
	if schema.Label != "" {
		fmt.Printf("  %s\n", schema.Label)
	}
	if schema.Description != "" {
		fmt.Printf("  %s\n", schema.Description)
	}

	writeOutpostDestinationTypeFields(os.Stdout, schema, true)
	fmt.Println()

	return nil
}

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type outpostEventGetCmd struct {
	cmd *cobra.Command

	tenantID string
	output   string
}

func newOutpostEventGetCmd() *outpostEventGetCmd {
	ec := &outpostEventGetCmd{}

	ec.cmd = &cobra.Command{
		Use:     "get <event-id>",
		Args:    validators.ExactArgs(1),
		Short:   ShortGet(ResourceEvent),
		Long:    `Get an event, including the payload that was published.`,
		PreRunE: ec.validateFlags,
		RunE:    ec.run,
		Example: `  # Get an event
  hookdeck outpost event get evt_abc123

  # Get the payload alone
  hookdeck outpost event get evt_abc123 --output json | jq .data`,
		Annotations: map[string]string{
			"cli.arguments": `[
				{"name":"event-id","type":"string","description":"The ID of the event.","required":true}
			]`,
		},
	}

	ec.cmd.Flags().StringVar(&ec.tenantID, "tenant-id", "", "Tenant the event belongs to")
	ec.cmd.Flags().StringVar(&ec.output, "output", "", "Output format (json)")

	return ec
}

func (ec *outpostEventGetCmd) validateFlags(cmd *cobra.Command, args []string) error {
	return rejectEmptyFlags(cmd)
}

func (ec *outpostEventGetCmd) run(cmd *cobra.Command, args []string) error {
	client := Config.GetOutpostAPIClient()

	event, err := client.GetOutpostEvent(context.Background(), args[0], ec.tenantID)
	if err != nil {
		return fmt.Errorf("failed to get event: %w", err)
	}

	if ec.output == "json" {
		return printJSONIndented(event)
	}

	color := ansi.Color(os.Stdout)
	fmt.Printf("\n%s\n", color.Green(event.ID))
	fmt.Printf("  Topic: %s\n", event.Topic)
	fmt.Printf("  Tenant: %s\n", event.TenantID)
	if len(event.MatchedDestinationIDs) > 0 {
		fmt.Printf("  Destinations: %s\n", strings.Join(event.MatchedDestinationIDs, ", "))
	}
	fmt.Printf("  Time: %s\n", event.Time.Format("2006-01-02 15:04:05"))
	for key, value := range event.Metadata {
		fmt.Printf("  Metadata %s: %s\n", key, value)
	}
	if len(event.Data) > 0 {
		if payload, err := json.MarshalIndent(event.Data, "  ", "  "); err == nil {
			fmt.Printf("  Data: %s\n", string(payload))
		}
	}
	fmt.Println()

	return nil
}

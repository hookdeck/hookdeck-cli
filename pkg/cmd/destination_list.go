package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type destinationListCmd struct {
	cmd *cobra.Command

	name     string
	destType string
	disabled bool
	limit    int
	output   string
}

func newDestinationListCmd() *destinationListCmd {
	dc := &destinationListCmd{}

	dc.cmd = &cobra.Command{
		Use:   "list",
		Args:  validators.NoArgs,
		Short: ShortList(ResourceDestination),
		Long: `List all destinations or filter by name or type.

Examples:
  hookdeck gateway destination list
  hookdeck gateway destination list --name my-destination
  hookdeck gateway destination list --type HTTP
  hookdeck gateway destination list --disabled
  hookdeck gateway destination list --limit 10`,
		RunE: dc.runDestinationListCmd,
	}

	dc.cmd.Flags().StringVar(&dc.name, "name", "", "Filter by destination name")
	dc.cmd.Flags().StringVar(&dc.destType, "type", "", "Filter by destination type (HTTP, CLI, MOCK_API)")
	dc.cmd.Flags().BoolVar(&dc.disabled, "disabled", false, "Include disabled destinations")
	dc.cmd.Flags().IntVar(&dc.limit, "limit", 100, "Limit number of results")
	dc.cmd.Flags().StringVar(&dc.output, "output", "", "Output format (json)")

	return dc
}

func (dc *destinationListCmd) runDestinationListCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	client := Config.GetAPIClient()
	params := make(map[string]string)

	if dc.name != "" {
		params["name"] = dc.name
	}
	if dc.destType != "" {
		params["type"] = dc.destType
	}
	if dc.disabled {
		params["disabled"] = "true"
	} else {
		params["disabled"] = "false"
	}
	params["limit"] = strconv.Itoa(dc.limit)

	resp, err := client.ListDestinations(context.Background(), params)
	if err != nil {
		return fmt.Errorf("failed to list destinations: %w", err)
	}

	if dc.output == "json" {
		if len(resp.Models) == 0 {
			fmt.Println("[]")
			return nil
		}
		jsonBytes, err := json.MarshalIndent(resp.Models, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal destinations to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	if len(resp.Models) == 0 {
		fmt.Println("No destinations found.")
		return nil
	}

	color := ansi.Color(os.Stdout)
	fmt.Printf("\nFound %d destination(s):\n\n", len(resp.Models))
	for _, d := range resp.Models {
		fmt.Printf("%s\n", color.Green(d.Name))
		fmt.Printf("  ID: %s\n", d.ID)
		fmt.Printf("  Type: %s\n", d.Type)
		if url := d.GetHTTPURL(); url != nil {
			fmt.Printf("  URL: %s\n", *url)
		}
		if d.DisabledAt != nil {
			fmt.Printf("  Status: %s\n", color.Red("disabled"))
		} else {
			fmt.Printf("  Status: %s\n", color.Green("active"))
		}
		fmt.Println()
	}

	return nil
}

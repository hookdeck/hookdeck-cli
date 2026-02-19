package cmd

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type destinationCountCmd struct {
	cmd *cobra.Command

	name     string
	destType string
	disabled bool
}

func newDestinationCountCmd() *destinationCountCmd {
	dc := &destinationCountCmd{}

	dc.cmd = &cobra.Command{
		Use:   "count",
		Args:  validators.NoArgs,
		Short: "Count destinations",
		Long: `Count destinations matching optional filters.

Examples:
  hookdeck gateway destination count
  hookdeck gateway destination count --type HTTP
  hookdeck gateway destination count --disabled`,
		RunE: dc.runDestinationCountCmd,
	}

	dc.cmd.Flags().StringVar(&dc.name, "name", "", "Filter by destination name")
	dc.cmd.Flags().StringVar(&dc.destType, "type", "", "Filter by destination type (HTTP, CLI, MOCK_API)")
	dc.cmd.Flags().BoolVar(&dc.disabled, "disabled", false, "Count disabled destinations only (when set with other filters)")

	return dc
}

func (dc *destinationCountCmd) runDestinationCountCmd(cmd *cobra.Command, args []string) error {
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

	resp, err := client.CountDestinations(context.Background(), params)
	if err != nil {
		return fmt.Errorf("failed to count destinations: %w", err)
	}

	fmt.Println(strconv.Itoa(resp.Count))
	return nil
}

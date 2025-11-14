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

type connectionListCmd struct {
	cmd *cobra.Command

	name          string
	sourceID      string
	destinationID string
	disabled      bool
	limit         int
	output        string
}

func newConnectionListCmd() *connectionListCmd {
	cc := &connectionListCmd{}

	cc.cmd = &cobra.Command{
		Use:   "list",
		Args:  validators.NoArgs,
		Short: "List connections",
		Long: `List all connections or filter by source/destination.

Examples:
  # List all connections
  hookdeck connection list

  # Filter by connection name
  hookdeck connection list --name my-connection

  # Filter by source ID
  hookdeck connection list --source-id src_abc123

  # Filter by destination ID
  hookdeck connection list --destination-id dst_def456

  # Include disabled connections
  hookdeck connection list --disabled

  # Limit results
  hookdeck connection list --limit 10`,
		RunE: cc.runConnectionListCmd,
	}

	cc.cmd.Flags().StringVar(&cc.name, "name", "", "Filter by connection name")
	cc.cmd.Flags().StringVar(&cc.sourceID, "source-id", "", "Filter by source ID")
	cc.cmd.Flags().StringVar(&cc.destinationID, "destination-id", "", "Filter by destination ID")
	cc.cmd.Flags().BoolVar(&cc.disabled, "disabled", false, "Include disabled connections")
	cc.cmd.Flags().IntVar(&cc.limit, "limit", 100, "Limit number of results")
	cc.cmd.Flags().StringVar(&cc.output, "output", "", "Output format (json)")

	return cc
}

func (cc *connectionListCmd) runConnectionListCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	client := Config.GetAPIClient()

	// Build request parameters
	params := make(map[string]string)

	if cc.name != "" {
		params["name"] = cc.name
	}

	if cc.sourceID != "" {
		params["source_id"] = cc.sourceID
	}

	if cc.destinationID != "" {
		params["destination_id"] = cc.destinationID
	}

	// API behavior (tested in test-scripts/test-disabled-behavior.sh):
	// - NO parameter: Returns ALL connections (both active and disabled)
	// - disabled=false: Returns ONLY active connections (excludes disabled)
	// - disabled=true: Returns ALL connections (both active and disabled)
	//
	// CLI behavior (from test expectations):
	// - --disabled flag present: Include ALL connections (both active and disabled)
	// - --disabled flag absent: Include only active connections
	//
	// Therefore:
	// - When --disabled flag is PRESENT: Send disabled=true (to get all)
	// - When --disabled flag is ABSENT: Send disabled=false (to exclude disabled)
	if cc.disabled {
		params["disabled"] = "true"
	} else {
		params["disabled"] = "false"
	}

	params["limit"] = strconv.Itoa(cc.limit)

	// List connections
	response, err := client.ListConnections(context.Background(), params)
	if err != nil {
		return fmt.Errorf("failed to list connections: %w", err)
	}

	if cc.output == "json" {
		if len(response.Models) == 0 {
			// Print an empty JSON array
			fmt.Println("[]")
			return nil
		}
		jsonBytes, err := json.MarshalIndent(response.Models, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal connections to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	if len(response.Models) == 0 {
		fmt.Println("No connections found.")
		return nil
	}

	color := ansi.Color(os.Stdout)

	fmt.Printf("\nFound %d connection(s):\n\n", len(response.Models))
	for _, conn := range response.Models {
		connectionName := "unnamed"
		if conn.Name != nil {
			connectionName = *conn.Name
		}

		sourceName := "unknown"
		sourceID := "unknown"
		sourceType := "unknown"
		if conn.Source != nil {
			sourceName = conn.Source.Name
			sourceID = conn.Source.ID
			sourceType = conn.Source.Type
		}

		destinationName := "unknown"
		destinationID := "unknown"
		destinationType := "unknown"
		if conn.Destination != nil {
			destinationName = conn.Destination.Name
			destinationID = conn.Destination.ID
			destinationType = conn.Destination.Type
		}

		// Show connection name in color
		fmt.Printf("%s\n", color.Green(connectionName))
		fmt.Printf("  ID: %s\n", conn.ID)
		fmt.Printf("  Source: %s (%s) [%s]\n", sourceName, sourceID, sourceType)
		fmt.Printf("  Destination: %s (%s) [%s]\n", destinationName, destinationID, destinationType)

		if conn.DisabledAt != nil {
			fmt.Printf("  Status: %s\n", color.Red("disabled"))
		} else if conn.PausedAt != nil {
			fmt.Printf("  Status: %s\n", color.Yellow("paused"))
		} else {
			fmt.Printf("  Status: %s\n", color.Green("active"))
		}

		fmt.Println()
	}

	return nil
}

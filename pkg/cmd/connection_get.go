package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type connectionGetCmd struct {
	cmd *cobra.Command

	output string
}

func newConnectionGetCmd() *connectionGetCmd {
	cc := &connectionGetCmd{}

	cc.cmd = &cobra.Command{
		Use:   "get <connection-id>",
		Args:  validators.ExactArgs(1),
		Short: "Get connection details",
		Long: `Get detailed information about a specific connection.

Examples:
  # Get connection details
  hookdeck connection get conn_abc123`,
		RunE: cc.runConnectionGetCmd,
	}

	cc.cmd.Flags().StringVar(&cc.output, "output", "", "Output format (json)")

	return cc
}

func (cc *connectionGetCmd) runConnectionGetCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	connectionID := args[0]
	client := Config.GetAPIClient()
	ctx := context.Background()

	// Get connection by ID
	conn, err := client.GetConnection(ctx, connectionID)
	if err != nil {
		return fmt.Errorf("failed to get connection: %w", err)
	}

	if cc.output == "json" {
		jsonBytes, err := json.MarshalIndent(conn, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal connection to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
	} else {
		color := ansi.Color(os.Stdout)

		// Display connection details
		fmt.Printf("\n")

		connectionName := "unnamed"
		if conn.Name != nil {
			connectionName = *conn.Name
		}
		fmt.Printf("%s\n", color.Green(connectionName))

		fmt.Printf("  ID: %s\n", conn.ID)

		if conn.Description != nil && *conn.Description != "" {
			fmt.Printf("  Description: %s\n", *conn.Description)
		}

		if conn.FullName != nil {
			fmt.Printf("  Full Name: %s\n", *conn.FullName)
		}

		fmt.Printf("\n")

		// Source details
		if conn.Source != nil {
			fmt.Printf("Source:\n")
			fmt.Printf("  Name: %s\n", conn.Source.Name)
			fmt.Printf("  ID: %s\n", conn.Source.ID)
			fmt.Printf("  Type: %s\n", conn.Source.Type)
			fmt.Printf("  URL: %s\n", conn.Source.URL)
			fmt.Printf("\n")
		}

		// Destination details
		if conn.Destination != nil {
			fmt.Printf("Destination:\n")
			fmt.Printf("  Name: %s\n", conn.Destination.Name)
			fmt.Printf("  ID: %s\n", conn.Destination.ID)
			fmt.Printf("  Type: %s\n", conn.Destination.Type)

			if cliPath := conn.Destination.GetCLIPath(); cliPath != nil {
				fmt.Printf("  CLI Path: %s\n", *cliPath)
			}

			if httpURL := conn.Destination.GetHTTPURL(); httpURL != nil {
				fmt.Printf("  URL: %s\n", *httpURL)
			}
			fmt.Printf("\n")
		}

		// Status
		fmt.Printf("Status:\n")
		if conn.DisabledAt != nil {
			fmt.Printf("  %s (disabled at %s)\n", color.Red("Disabled"), conn.DisabledAt.Format("2006-01-02 15:04:05"))
		} else if conn.PausedAt != nil {
			fmt.Printf("  %s (paused at %s)\n", color.Yellow("Paused"), conn.PausedAt.Format("2006-01-02 15:04:05"))
		} else {
			fmt.Printf("  %s\n", color.Green("Active"))
		}
		fmt.Printf("\n")

		// Rules
		if len(conn.Rules) > 0 {
			fmt.Printf("Rules:\n")
			for i, rule := range conn.Rules {
				if ruleType, ok := rule["type"].(string); ok {
					fmt.Printf("  Rule %d: Type: %s\n", i+1, ruleType)
				}
			}
			fmt.Printf("\n")
		}

		// Timestamps
		fmt.Printf("Timestamps:\n")
		fmt.Printf("  Created: %s\n", conn.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Updated: %s\n", conn.UpdatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("\n")
	}

	return nil
}

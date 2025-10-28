package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type connectionUpdateCmd struct {
	cmd *cobra.Command

	name        string
	description string
	output      string
}

func newConnectionUpdateCmd() *connectionUpdateCmd {
	cc := &connectionUpdateCmd{}

	cc.cmd = &cobra.Command{
		Use:   "update <connection-id>",
		Args:  validators.ExactArgs(1),
		Short: "Update a connection",
		Long: `Update an existing connection's configuration.

Examples:
  # Update connection name
  hookdeck connection update conn_abc123 --name "new-name"

  # Update description
  hookdeck connection update conn_abc123 --description "Updated description"

  # Update both
  hookdeck connection update conn_abc123 --name "new-name" --description "New description"`,
		PreRunE: cc.validateFlags,
		RunE:    cc.runConnectionUpdateCmd,
	}

	cc.cmd.Flags().StringVar(&cc.name, "name", "", "Update connection name")
	cc.cmd.Flags().StringVar(&cc.description, "description", "", "Update connection description")
	cc.cmd.Flags().StringVar(&cc.output, "output", "", "Output format (json)")

	return cc
}

func (cc *connectionUpdateCmd) validateFlags(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	// Check that at least one update flag is provided
	if cc.name == "" && cc.description == "" {
		return fmt.Errorf("at least one update flag must be provided (--name or --description)")
	}

	return nil
}

func (cc *connectionUpdateCmd) runConnectionUpdateCmd(cmd *cobra.Command, args []string) error {
	connectionID := args[0]
	client := Config.GetAPIClient()
	ctx := context.Background()

	// Build update request
	req := &hookdeck.ConnectionUpdateRequest{}

	if cc.name != "" {
		req.Name = &cc.name
	}

	if cc.description != "" {
		req.Description = &cc.description
	}

	// Update connection
	connection, err := client.UpdateConnection(ctx, connectionID, req)
	if err != nil {
		return fmt.Errorf("failed to update connection: %w", err)
	}

	// Display results
	if cc.output == "json" {
		jsonBytes, err := json.MarshalIndent(connection, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal connection to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
	} else {
		fmt.Printf("\n✓ Connection updated successfully\n\n")

		connectionName := "unnamed"
		if connection.Name != nil {
			connectionName = *connection.Name
		}
		fmt.Printf("Connection: %s (%s)\n", connectionName, connection.ID)

		if connection.Description != nil && *connection.Description != "" {
			fmt.Printf("Description: %s\n", *connection.Description)
		}
	}

	return nil
}

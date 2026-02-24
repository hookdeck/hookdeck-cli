package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type connectionDeleteCmd struct {
	cmd *cobra.Command

	force bool
}

func newConnectionDeleteCmd() *connectionDeleteCmd {
	cc := &connectionDeleteCmd{}

	cc.cmd = &cobra.Command{
		Use:   "delete <connection-id>",
		Args:  validators.ExactArgs(1),
		Short: ShortDelete(ResourceConnection),
		Long: LongDeleteIntro(ResourceConnection) + `

Examples:
  # Delete a connection (with confirmation)
  hookdeck connection delete conn_abc123

  # Force delete without confirmation
  hookdeck connection delete conn_abc123 --force`,
		PreRunE: cc.validateFlags,
		RunE:    cc.runConnectionDeleteCmd,
	}

	cc.cmd.Flags().BoolVar(&cc.force, "force", false, "Force delete without confirmation")

	return cc
}

func (cc *connectionDeleteCmd) validateFlags(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	return nil
}

func (cc *connectionDeleteCmd) runConnectionDeleteCmd(cmd *cobra.Command, args []string) error {
	connectionID := args[0]
	client := Config.GetAPIClient()
	ctx := context.Background()

	// Get connection details first for confirmation
	conn, err := client.GetConnection(ctx, connectionID)
	if err != nil {
		return fmt.Errorf("failed to get connection: %w", err)
	}

	connectionName := "unnamed"
	if conn.Name != nil {
		connectionName = *conn.Name
	}

	// Confirm deletion unless --force is used
	if !cc.force {
		fmt.Printf("\nAre you sure you want to delete connection '%s' (%s)? [y/N]: ", connectionName, connectionID)
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Deletion cancelled.")
			return nil
		}
	}

	// Delete connection
	err = client.DeleteConnection(ctx, connectionID)
	if err != nil {
		return fmt.Errorf("failed to delete connection: %w", err)
	}

	fmt.Printf("\n"+SuccessCheck+" Connection '%s' (%s) deleted successfully\n", connectionName, connectionID)

	return nil
}

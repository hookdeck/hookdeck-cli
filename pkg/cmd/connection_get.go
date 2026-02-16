package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type connectionGetCmd struct {
	cmd *cobra.Command

	output      string
	includeDestinationAuth bool
}

func newConnectionGetCmd() *connectionGetCmd {
	cc := &connectionGetCmd{}

	cc.cmd = &cobra.Command{
		Use:   "get <connection-id-or-name>",
		Args:  validators.ExactArgs(1),
		Short: ShortGet(ResourceConnection),
		Long: LongGetIntro(ResourceConnection) + `

Examples:
	 # Get connection by ID
	 hookdeck connection get conn_abc123
	 
	 # Get connection by name
	 hookdeck connection get my-connection`,
		RunE: cc.runConnectionGetCmd,
	}

	cc.cmd.Flags().StringVar(&cc.output, "output", "", "Output format (json)")
	addIncludeDestinationAuthFlag(cc.cmd, &cc.includeDestinationAuth)

	return cc
}

func (cc *connectionGetCmd) runConnectionGetCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	connectionIDOrName := args[0]
	apiClient := Config.GetAPIClient()
	ctx := context.Background()

	// Resolve connection ID from name or ID
	connectionID, err := resolveConnectionID(ctx, apiClient, connectionIDOrName)
	if err != nil {
		return err
	}

	// Get connection by ID
	conn, err := apiClient.GetConnection(ctx, connectionID)
	if err != nil {
		return formatConnectionError(err, connectionIDOrName)
	}

	// The connections API does not support include=config.auth, so when
	// --include-destination-auth is requested we fetch the destination directly
	// from GET /destinations/{id}?include=config.auth and merge the enriched
	// config back into the connection response.
	if cc.includeDestinationAuth && conn.Destination != nil {
		dest, err := apiClient.GetDestination(ctx, conn.Destination.ID, includeAuthParams(true))
		if err == nil {
			conn.Destination = dest
		}
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

// resolveConnectionID accepts both connection names and IDs
// Try as ID first (if it starts with conn_ or web_), then lookup by name
func resolveConnectionID(ctx context.Context, client *hookdeck.Client, nameOrID string) (string, error) {
	// If it looks like a connection ID, try it directly
	if strings.HasPrefix(nameOrID, "conn_") || strings.HasPrefix(nameOrID, "web_") {
		// Try to get it to verify it exists
		_, err := client.GetConnection(ctx, nameOrID)
		if err == nil {
			return nameOrID, nil
		}
		// If we get a 404, fall through to name lookup
		// For other errors, format and return the error
		errMsg := strings.ToLower(err.Error())
		if !strings.Contains(errMsg, "404") && !strings.Contains(errMsg, "not found") {
			return "", err
		}
		// 404 on ID lookup - fall through to try name lookup
	}

	// Try to find by name
	params := map[string]string{
		"name": nameOrID,
	}

	result, err := client.ListConnections(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to lookup connection by name '%s': %w", nameOrID, err)
	}

	if result.Pagination.Limit == 0 || len(result.Models) == 0 {
		return "", fmt.Errorf("connection not found: '%s'\n\nPlease check the connection name or ID and try again", nameOrID)
	}

	if len(result.Models) > 1 {
		return "", fmt.Errorf("multiple connections found with name '%s', please use the connection ID instead", nameOrID)
	}

	return result.Models[0].ID, nil
}

// formatConnectionError provides user-friendly error messages for connection get failures
func formatConnectionError(err error, identifier string) error {
	errMsg := err.Error()

	// Check for 404/not found errors (case-insensitive)
	errMsgLower := strings.ToLower(errMsg)
	if strings.Contains(errMsgLower, "404") || strings.Contains(errMsgLower, "not found") {
		return fmt.Errorf("connection not found: '%s'\n\nPlease check the connection name or ID and try again", identifier)
	}

	// Check for network/timeout errors
	if strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "connection refused") {
		return fmt.Errorf("failed to connect to Hookdeck API: %w\n\nPlease check your network connection and try again", err)
	}

	// Default to the original error with some context
	return fmt.Errorf("failed to get connection '%s': %w", identifier, err)
}

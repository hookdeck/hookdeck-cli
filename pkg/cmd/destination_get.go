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

type destinationGetCmd struct {
	cmd *cobra.Command

	output            string
	includeDestAuth   bool
}

func newDestinationGetCmd() *destinationGetCmd {
	dc := &destinationGetCmd{}

	dc.cmd = &cobra.Command{
		Use:   "get <destination-id-or-name>",
		Args:  validators.ExactArgs(1),
		Short: ShortGet(ResourceDestination),
		Long: LongGetIntro(ResourceDestination) + `

Examples:
  hookdeck gateway destination get des_abc123
  hookdeck gateway destination get my-destination --include-auth`,
		RunE: dc.runDestinationGetCmd,
	}

	dc.cmd.Flags().StringVar(&dc.output, "output", "", "Output format (json)")
	addIncludeAuthFlagForDestination(dc.cmd, &dc.includeDestAuth)

	return dc
}

func (dc *destinationGetCmd) runDestinationGetCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	idOrName := args[0]
	client := Config.GetAPIClient()
	ctx := context.Background()

	destID, err := resolveDestinationID(ctx, client, idOrName)
	if err != nil {
		return err
	}

	params := includeAuthParams(dc.includeDestAuth)

	dst, err := client.GetDestination(ctx, destID, params)
	if err != nil {
		return fmt.Errorf("failed to get destination: %w", err)
	}

	if dc.output == "json" {
		jsonBytes, err := json.MarshalIndent(dst, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal destination to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	color := ansi.Color(os.Stdout)
	fmt.Printf("\n%s\n", color.Green(dst.Name))
	fmt.Printf("  ID: %s\n", dst.ID)
	fmt.Printf("  Type: %s\n", dst.Type)
	if url := dst.GetHTTPURL(); url != nil {
		fmt.Printf("  URL: %s\n", *url)
	}
	if path := dst.GetCLIPath(); path != nil {
		fmt.Printf("  Path: %s\n", *path)
	}
	if dst.Description != nil && *dst.Description != "" {
		fmt.Printf("  Description: %s\n", *dst.Description)
	}
	if dst.DisabledAt != nil {
		fmt.Printf("  Status: %s\n", color.Red("disabled"))
	} else {
		fmt.Printf("  Status: %s\n", color.Green("active"))
	}
	fmt.Printf("  Created: %s\n", dst.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("  Updated: %s\n", dst.UpdatedAt.Format("2006-01-02 15:04:05"))
	fmt.Println()

	return nil
}

// resolveDestinationID returns the destination ID for the given name or ID.
func resolveDestinationID(ctx context.Context, client *hookdeck.Client, nameOrID string) (string, error) {
	if strings.HasPrefix(nameOrID, "des_") {
		_, err := client.GetDestination(ctx, nameOrID, nil)
		if err == nil {
			return nameOrID, nil
		}
		if !hookdeck.IsNotFoundError(err) {
			return "", err
		}
	}

	params := map[string]string{"name": nameOrID}
	result, err := client.ListDestinations(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to lookup destination by name '%s': %w", nameOrID, err)
	}
	if len(result.Models) == 0 {
		return "", fmt.Errorf("no destination found with name or ID '%s'", nameOrID)
	}
	return result.Models[0].ID, nil
}

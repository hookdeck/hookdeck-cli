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

type sourceGetCmd struct {
	cmd              *cobra.Command
	output           string
	includeAuth      bool
}

func newSourceGetCmd() *sourceGetCmd {
	sc := &sourceGetCmd{}

	sc.cmd = &cobra.Command{
		Use:   "get <source-id-or-name>",
		Args:  validators.ExactArgs(1),
		Short: ShortGet(ResourceSource),
		Long: LongGetIntro(ResourceSource) + `

Examples:
  hookdeck gateway source get src_abc123
  hookdeck gateway source get my-source --include-auth`,
		RunE: sc.runSourceGetCmd,
	}

	sc.cmd.Flags().StringVar(&sc.output, "output", "", "Output format (json)")
	addIncludeSourceAuthFlag(sc.cmd, &sc.includeAuth)

	return sc
}

func (sc *sourceGetCmd) runSourceGetCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	idOrName := args[0]
	client := Config.GetAPIClient()
	ctx := context.Background()

	sourceID, err := resolveSourceID(ctx, client, idOrName)
	if err != nil {
		return err
	}

	params := includeAuthParams(sc.includeAuth)

	src, err := client.GetSource(ctx, sourceID, params)
	if err != nil {
		return fmt.Errorf("failed to get source: %w", err)
	}

	if sc.output == "json" {
		jsonBytes, err := json.MarshalIndent(src, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal source to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	color := ansi.Color(os.Stdout)
	fmt.Printf("\n%s\n", color.Green(src.Name))
	fmt.Printf("  ID: %s\n", src.ID)
	fmt.Printf("  Type: %s\n", src.Type)
	fmt.Printf("  URL: %s\n", src.URL)
	if src.Description != nil && *src.Description != "" {
		fmt.Printf("  Description: %s\n", *src.Description)
	}
	if src.DisabledAt != nil {
		fmt.Printf("  Status: %s\n", color.Red("disabled"))
	} else {
		fmt.Printf("  Status: %s\n", color.Green("active"))
	}
	fmt.Printf("  Created: %s\n", src.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("  Updated: %s\n", src.UpdatedAt.Format("2006-01-02 15:04:05"))
	fmt.Println()

	return nil
}

// resolveSourceID returns the source ID for the given name or ID.
func resolveSourceID(ctx context.Context, client *hookdeck.Client, nameOrID string) (string, error) {
	if strings.HasPrefix(nameOrID, "src_") {
		_, err := client.GetSource(ctx, nameOrID, nil)
		if err == nil {
			return nameOrID, nil
		}
		errMsg := strings.ToLower(err.Error())
		if !strings.Contains(errMsg, "404") && !strings.Contains(errMsg, "not found") {
			return "", err
		}
	}

	params := map[string]string{"name": nameOrID}
	result, err := client.ListSources(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to lookup source by name '%s': %w", nameOrID, err)
	}
	if result.Models == nil || len(result.Models) == 0 {
		return "", fmt.Errorf("no source found with name or ID '%s'", nameOrID)
	}
	return result.Models[0].ID, nil
}

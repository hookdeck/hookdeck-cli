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

type transformationGetCmd struct {
	cmd    *cobra.Command
	output string
}

func newTransformationGetCmd() *transformationGetCmd {
	tc := &transformationGetCmd{}

	tc.cmd = &cobra.Command{
		Use:   "get <transformation-id-or-name>",
		Args:  validators.ExactArgs(1),
		Short: ShortGet(ResourceTransformation),
		Long:  LongGetIntro(ResourceTransformation) + `

Examples:
  hookdeck gateway transformation get trn_abc123
  hookdeck gateway transformation get my-transform`,
		RunE: tc.runTransformationGetCmd,
	}

	tc.cmd.Flags().StringVar(&tc.output, "output", "", "Output format (json)")

	return tc
}

func (tc *transformationGetCmd) runTransformationGetCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	idOrName := args[0]
	client := Config.GetAPIClient()
	ctx := context.Background()

	trnID, err := resolveTransformationID(ctx, client, idOrName)
	if err != nil {
		return err
	}

	t, err := client.GetTransformation(ctx, trnID)
	if err != nil {
		return fmt.Errorf("failed to get transformation: %w", err)
	}

	if tc.output == "json" {
		jsonBytes, err := json.MarshalIndent(t, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal transformation to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	color := ansi.Color(os.Stdout)
	fmt.Printf("\n%s\n", color.Green(t.Name))
	fmt.Printf("  ID:   %s\n", t.ID)
	fmt.Printf("  Code: %s\n", truncate(t.Code, 80))
	if len(t.Env) > 0 {
		fmt.Printf("  Env:  %v\n", t.Env)
	}
	fmt.Printf("  Created: %s\n", t.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("  Updated: %s\n", t.UpdatedAt.Format("2006-01-02 15:04:05"))
	fmt.Println()

	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// resolveTransformationID returns the transformation ID for the given name or ID.
func resolveTransformationID(ctx context.Context, client *hookdeck.Client, nameOrID string) (string, error) {
	// If it looks like an ID (e.g. trs_xxx), try Get first
	if strings.HasPrefix(nameOrID, "trs_") {
		_, err := client.GetTransformation(ctx, nameOrID)
		if err == nil {
			return nameOrID, nil
		}
		errMsg := strings.ToLower(err.Error())
		if !strings.Contains(errMsg, "404") && !strings.Contains(errMsg, "not found") {
			return "", err
		}
	}

	params := map[string]string{"name": nameOrID}
	result, err := client.ListTransformations(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to lookup transformation by name '%s': %w", nameOrID, err)
	}
	if len(result.Models) == 0 {
		return "", fmt.Errorf("no transformation found with name or ID '%s'", nameOrID)
	}
	return result.Models[0].ID, nil
}

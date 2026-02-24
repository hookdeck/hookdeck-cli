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

type requestGetCmd struct {
	cmd    *cobra.Command
	output string
}

func newRequestGetCmd() *requestGetCmd {
	rc := &requestGetCmd{}

	rc.cmd = &cobra.Command{
		Use:   "get <request-id>",
		Args:  validators.ExactArgs(1),
		Short: ShortGet(ResourceRequest),
		Long: `Get detailed information about a request by ID.

Examples:
  hookdeck gateway request get req_abc123`,
		RunE: rc.runRequestGetCmd,
	}

	rc.cmd.Flags().StringVar(&rc.output, "output", "", "Output format (json)")

	return rc
}

func (rc *requestGetCmd) runRequestGetCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	requestID := args[0]
	client := Config.GetAPIClient()
	ctx := context.Background()

	req, err := client.GetRequest(ctx, requestID, nil)
	if err != nil {
		return fmt.Errorf("failed to get request: %w", err)
	}

	if rc.output == "json" {
		jsonBytes, err := json.MarshalIndent(req, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal request to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	color := ansi.Color(os.Stdout)
	fmt.Printf("\n%s\n", color.Green(req.ID))
	fmt.Printf("  Source ID:     %s\n", req.SourceID)
	fmt.Printf("  Verified:      %v\n", req.Verified)
	fmt.Printf("  Events count:  %d\n", req.EventsCount)
	fmt.Printf("  Ignored count: %d\n", req.IgnoredCount)
	if req.RejectionCause != nil {
		fmt.Printf("  Rejection:     %s\n", *req.RejectionCause)
	}
	fmt.Printf("  Created:       %s\n", req.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Println()
	return nil
}

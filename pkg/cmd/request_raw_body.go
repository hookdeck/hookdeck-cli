package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type requestRawBodyCmd struct {
	cmd *cobra.Command
}

func newRequestRawBodyCmd() *requestRawBodyCmd {
	rc := &requestRawBodyCmd{}

	rc.cmd = &cobra.Command{
		Use:   "raw-body <request-id>",
		Args:  validators.ExactArgs(1),
		Short: "Get raw body of a request",
		Long: `Output the raw request body of a request by ID.

Examples:
  hookdeck gateway request raw-body req_abc123`,
		RunE: rc.runRequestRawBodyCmd,
	}

	return rc
}

func (rc *requestRawBodyCmd) runRequestRawBodyCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	requestID := args[0]
	client := Config.GetAPIClient()
	ctx := context.Background()

	body, err := client.GetRequestRawBody(ctx, requestID)
	if err != nil {
		return fmt.Errorf("failed to get request raw body: %w", err)
	}
	_, _ = os.Stdout.Write(body)
	return nil
}

package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type requestRetryCmd struct {
	cmd            *cobra.Command
	connectionIDs  string
}

func newRequestRetryCmd() *requestRetryCmd {
	rc := &requestRetryCmd{}

	rc.cmd = &cobra.Command{
		Use:   "retry <request-id>",
		Args:  validators.ExactArgs(1),
		Short: "Retry a request",
		Long: `Retry a request by ID. By default retries on all connections. Use --connection-ids to retry only for specific connections.

Examples:
  hookdeck gateway request retry req_abc123
  hookdeck gateway request retry req_abc123 --connection-ids web_1,web_2`,
		RunE: rc.runRequestRetryCmd,
	}

	rc.cmd.Flags().StringVar(&rc.connectionIDs, "connection-ids", "", "Comma-separated connection IDs to retry (omit to retry all)")

	return rc
}

func (rc *requestRetryCmd) runRequestRetryCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	requestID := args[0]
	client := Config.GetAPIClient()
	ctx := context.Background()

	body := &hookdeck.RequestRetryRequest{}
	if rc.connectionIDs != "" {
		body.WebhookIDs = strings.Split(rc.connectionIDs, ",")
		for i, id := range body.WebhookIDs {
			body.WebhookIDs[i] = strings.TrimSpace(id)
		}
	}

	if err := client.RetryRequest(ctx, requestID, body); err != nil {
		return fmt.Errorf("failed to retry request: %w", err)
	}
	fmt.Printf("Request %s retry requested.\n", requestID)
	return nil
}

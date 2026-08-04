package cmd

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type sourceCountCmd struct {
	cmd *cobra.Command

	name     string
	sourceType string
	disabled bool
}

func newSourceCountCmd() *sourceCountCmd {
	sc := &sourceCountCmd{}

	sc.cmd = &cobra.Command{
		Use:   "count",
		Args:  validators.NoArgs,
		Short: "Count sources",
		Long: `Count sources matching optional filters.

Examples:
  hookdeck gateway source count
  hookdeck gateway source count --type WEBHOOK
  hookdeck gateway source count --disabled`,
		RunE: sc.runSourceCountCmd,
	}

	sc.cmd.Flags().StringVar(&sc.name, "name", "", "Filter by source name")
	sc.cmd.Flags().StringVar(&sc.sourceType, "type", "", "Filter by source type")
	sc.cmd.Flags().BoolVar(&sc.disabled, "disabled", false, "Count disabled sources only (when set with other filters)")

	return sc
}

func (sc *sourceCountCmd) runSourceCountCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	client := Config.GetAPIClient()
	params := make(map[string]string)
	if sc.name != "" {
		params["name"] = sc.name
	}
	if sc.sourceType != "" {
		params["type"] = sc.sourceType
	}
	if sc.disabled {
		params["disabled"] = "true"
	} else {
		params["disabled"] = "false"
	}

	resp, err := client.CountSources(context.Background(), params)
	if err != nil {
		return fmt.Errorf("failed to count sources: %w", err)
	}

	fmt.Println(strconv.Itoa(resp.Count))
	return nil
}

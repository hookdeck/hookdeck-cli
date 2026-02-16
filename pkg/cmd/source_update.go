package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type sourceUpdateCmd struct {
	cmd *cobra.Command

	name        string
	description string
	sourceType  string
	config      string
	configFile  string
	output      string
}

func newSourceUpdateCmd() *sourceUpdateCmd {
	sc := &sourceUpdateCmd{}

	sc.cmd = &cobra.Command{
		Use:   "update <source-id>",
		Args:  validators.ExactArgs(1),
		Short: "Update a source by ID",
		Long: `Update an existing source by its ID.

Examples:
  hookdeck gateway source update src_abc123 --name new-name
  hookdeck gateway source update src_abc123 --description "Updated"
  hookdeck gateway source update src_abc123 --config '{"webhook_secret":"whsec_new"}'`,
		PreRunE: sc.validateFlags,
		RunE:    sc.runSourceUpdateCmd,
	}

	sc.cmd.Flags().StringVar(&sc.name, "name", "", "New source name")
	sc.cmd.Flags().StringVar(&sc.description, "description", "", "New source description")
	sc.cmd.Flags().StringVar(&sc.sourceType, "type", "", "Source type (e.g. WEBHOOK, STRIPE)")
	sc.cmd.Flags().StringVar(&sc.config, "config", "", "JSON object for source config")
	sc.cmd.Flags().StringVar(&sc.configFile, "config-file", "", "Path to JSON file for source config")
	sc.cmd.Flags().StringVar(&sc.output, "output", "", "Output format (json)")

	return sc
}

func (sc *sourceUpdateCmd) validateFlags(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	if sc.config != "" && sc.configFile != "" {
		return fmt.Errorf("cannot use both --config and --config-file")
	}
	return nil
}

func (sc *sourceUpdateCmd) runSourceUpdateCmd(cmd *cobra.Command, args []string) error {
	sourceID := args[0]
	client := Config.GetAPIClient()
	ctx := context.Background()

	// Build update request from flags (only set non-zero values)
	req := &hookdeck.SourceCreateRequest{}
	req.Name = sc.name
	if sc.description != "" {
		req.Description = &sc.description
	}
	if sc.sourceType != "" {
		req.Type = strings.ToUpper(sc.sourceType)
	}
	config, err := buildSourceConfigFromFlags(sc.config, sc.configFile)
	if err != nil {
		return err
	}
	if len(config) > 0 {
		req.Config = config
	}

	// If no fields set, fetch current and re-send name at least (API may require name)
	if req.Name == "" && req.Description == nil && req.Type == "" && len(req.Config) == 0 {
		existing, err := client.GetSource(ctx, sourceID, nil)
		if err != nil {
			return fmt.Errorf("failed to get source: %w", err)
		}
		req.Name = existing.Name
	}

	src, err := client.UpdateSource(ctx, sourceID, req)
	if err != nil {
		return fmt.Errorf("failed to update source: %w", err)
	}

	if sc.output == "json" {
		jsonBytes, err := json.MarshalIndent(src, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal source to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	fmt.Printf("✔ Source updated successfully\n\n")
	fmt.Printf("Source: %s (%s)\n", src.Name, src.ID)
	fmt.Printf("Type:  %s\n", src.Type)
	fmt.Printf("URL:   %s\n", src.URL)
	return nil
}

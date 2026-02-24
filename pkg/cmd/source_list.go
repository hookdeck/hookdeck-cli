package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type sourceListCmd struct {
	cmd *cobra.Command

	name     string
	sourceType string
	disabled bool
	limit    int
	output   string
}

func newSourceListCmd() *sourceListCmd {
	sc := &sourceListCmd{}

	sc.cmd = &cobra.Command{
		Use:   "list",
		Args:  validators.NoArgs,
		Short: ShortList(ResourceSource),
		Long: `List all sources or filter by name or type.

Examples:
  hookdeck gateway source list
  hookdeck gateway source list --name my-source
  hookdeck gateway source list --type WEBHOOK
  hookdeck gateway source list --disabled
  hookdeck gateway source list --limit 10`,
		RunE: sc.runSourceListCmd,
	}

	sc.cmd.Flags().StringVar(&sc.name, "name", "", "Filter by source name")
	sc.cmd.Flags().StringVar(&sc.sourceType, "type", "", "Filter by source type (e.g. WEBHOOK, STRIPE)")
	sc.cmd.Flags().BoolVar(&sc.disabled, "disabled", false, "Include disabled sources")
	sc.cmd.Flags().IntVar(&sc.limit, "limit", 100, "Limit number of results")
	sc.cmd.Flags().StringVar(&sc.output, "output", "", "Output format (json)")

	return sc
}

func (sc *sourceListCmd) runSourceListCmd(cmd *cobra.Command, args []string) error {
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
	params["limit"] = strconv.Itoa(sc.limit)

	resp, err := client.ListSources(context.Background(), params)
	if err != nil {
		return fmt.Errorf("failed to list sources: %w", err)
	}

	if sc.output == "json" {
		jsonBytes, err := marshalListResponseWithPagination(resp.Models, resp.Pagination)
		if err != nil {
			return fmt.Errorf("failed to marshal sources to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	if len(resp.Models) == 0 {
		fmt.Println("No sources found.")
		return nil
	}

	color := ansi.Color(os.Stdout)
	fmt.Printf("\nFound %d source(s):\n\n", len(resp.Models))
	for _, src := range resp.Models {
		fmt.Printf("%s\n", color.Green(src.Name))
		fmt.Printf("  ID: %s\n", src.ID)
		fmt.Printf("  Type: %s\n", src.Type)
		fmt.Printf("  URL: %s\n", src.URL)
		if src.DisabledAt != nil {
			fmt.Printf("  Status: %s\n", color.Red("disabled"))
		} else {
			fmt.Printf("  Status: %s\n", color.Green("active"))
		}
		fmt.Println()
	}

	// Display pagination info
	commandExample := "hookdeck gateway source list"
	printPaginationInfo(resp.Pagination, commandExample)

	return nil
}

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type transformationListCmd struct {
	cmd *cobra.Command

	id       string
	name     string
	orderBy  string
	dir      string
	limit    int
	next     string
	prev     string
	output   string
}

func newTransformationListCmd() *transformationListCmd {
	tc := &transformationListCmd{}

	tc.cmd = &cobra.Command{
		Use:   "list",
		Args:  validators.NoArgs,
		Short: ShortList(ResourceTransformation),
		Long: `List all transformations or filter by name or id.

Examples:
  hookdeck gateway transformation list
  hookdeck gateway transformation list --name my-transform
  hookdeck gateway transformation list --order-by created_at --dir desc
  hookdeck gateway transformation list --limit 10`,
		RunE: tc.runTransformationListCmd,
	}

	tc.cmd.Flags().StringVar(&tc.id, "id", "", "Filter by transformation ID(s)")
	tc.cmd.Flags().StringVar(&tc.name, "name", "", "Filter by transformation name")
	tc.cmd.Flags().StringVar(&tc.orderBy, "order-by", "", "Sort key (name, created_at, updated_at)")
	tc.cmd.Flags().StringVar(&tc.dir, "dir", "", "Sort direction (asc, desc)")
	tc.cmd.Flags().IntVar(&tc.limit, "limit", 100, "Limit number of results")
	tc.cmd.Flags().StringVar(&tc.next, "next", "", "Pagination cursor for next page")
	tc.cmd.Flags().StringVar(&tc.prev, "prev", "", "Pagination cursor for previous page")
	tc.cmd.Flags().StringVar(&tc.output, "output", "", "Output format (json)")

	return tc
}

func (tc *transformationListCmd) runTransformationListCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	client := Config.GetAPIClient()
	params := make(map[string]string)
	if tc.id != "" {
		params["id"] = tc.id
	}
	if tc.name != "" {
		params["name"] = tc.name
	}
	if tc.orderBy != "" {
		params["order_by"] = tc.orderBy
	}
	if tc.dir != "" {
		params["dir"] = tc.dir
	}
	params["limit"] = strconv.Itoa(tc.limit)
	if tc.next != "" {
		params["next"] = tc.next
	}
	if tc.prev != "" {
		params["prev"] = tc.prev
	}

	resp, err := client.ListTransformations(context.Background(), params)
	if err != nil {
		return fmt.Errorf("failed to list transformations: %w", err)
	}

	if tc.output == "json" {
		if len(resp.Models) == 0 {
			fmt.Println("[]")
			return nil
		}
		jsonBytes, err := json.MarshalIndent(resp.Models, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal transformations to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	if len(resp.Models) == 0 {
		fmt.Println("No transformations found.")
		return nil
	}

	color := ansi.Color(os.Stdout)
	for _, t := range resp.Models {
		fmt.Printf("%s %s\n", color.Green(t.Name), t.ID)
	}
	return nil
}

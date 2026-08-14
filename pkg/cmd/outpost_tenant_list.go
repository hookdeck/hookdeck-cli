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

type outpostTenantListCmd struct {
	cmd *cobra.Command

	ids    string
	limit  int
	dir    string
	next   string
	prev   string
	output string
}

func newOutpostTenantListCmd() *outpostTenantListCmd {
	tc := &outpostTenantListCmd{}

	tc.cmd = &cobra.Command{
		Use:     "list",
		Args:    validators.NoArgs,
		Short:   ShortList(ResourceTenant),
		Long:    `List tenants in the current Outpost project.`,
		PreRunE: tc.validateFlags,
		RunE:    tc.runOutpostTenantListCmd,
		Example: `  # List tenants
  hookdeck outpost tenant list

  # Fetch specific tenants by ID
  hookdeck outpost tenant list --id acme,globex

  # Page through results
  hookdeck outpost tenant list --limit 20 --next <cursor>`,
	}

	tc.cmd.Flags().StringVar(&tc.ids, "id", "", "Filter by tenant ID(s), comma-separated")
	tc.cmd.Flags().IntVar(&tc.limit, "limit", 0, "Limit number of results (1-100)")
	tc.cmd.Flags().StringVar(&tc.dir, "dir", "", "Sort direction (asc, desc)")
	tc.cmd.Flags().StringVar(&tc.next, "next", "", "Next page cursor")
	tc.cmd.Flags().StringVar(&tc.prev, "prev", "", "Previous page cursor")
	tc.cmd.Flags().StringVar(&tc.output, "output", "", "Output format (json)")

	return tc
}

func (tc *outpostTenantListCmd) validateFlags(cmd *cobra.Command, args []string) error {
	return rejectEmptyFlags(cmd)
}

func (tc *outpostTenantListCmd) runOutpostTenantListCmd(cmd *cobra.Command, args []string) error {
	client := Config.GetOutpostAPIClient()

	resp, err := client.ListOutpostTenants(context.Background(), hookdeck.OutpostTenantListParams{
		IDs:   splitCommaList(tc.ids),
		Limit: tc.limit,
		Dir:   tc.dir,
		Next:  tc.next,
		Prev:  tc.prev,
	})
	if err != nil {
		return fmt.Errorf("failed to list tenants: %w", err)
	}

	if tc.output == "json" {
		jsonBytes, err := marshalListResponseWithPagination(resp.Models, resp.Pagination)
		if err != nil {
			return fmt.Errorf("failed to marshal tenants to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	if len(resp.Models) == 0 {
		fmt.Println("No tenants found.")
		return nil
	}

	color := ansi.Color(os.Stdout)
	fmt.Printf("\nFound %d tenant(s):\n\n", len(resp.Models))
	for _, tenant := range resp.Models {
		fmt.Printf("%s\n", color.Green(tenant.ID))
		fmt.Printf("  Destinations: %d\n", tenant.DestinationsCount)
		if len(tenant.Topics) > 0 {
			fmt.Printf("  Topics: %s\n", strings.Join(tenant.Topics, ", "))
		}
		fmt.Printf("  Created: %s\n", tenant.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Println()
	}

	printPaginationInfo(resp.Pagination, "hookdeck outpost tenant list")

	return nil
}

// splitCommaList turns a comma-separated flag value into a slice, dropping
// empty entries so a trailing comma does not produce a blank filter.
func splitCommaList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// printJSONIndented is the shared single-object JSON output path.
func printJSONIndented(v interface{}) error {
	jsonBytes, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal to json: %w", err)
	}
	fmt.Println(string(jsonBytes))
	return nil
}

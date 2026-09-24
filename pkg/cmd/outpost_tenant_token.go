package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type outpostTenantTokenCmd struct {
	cmd *cobra.Command

	output string
}

func newOutpostTenantTokenCmd() *outpostTenantTokenCmd {
	tc := &outpostTenantTokenCmd{}

	tc.cmd = &cobra.Command{
		Use:   "token <tenant-id>",
		Args:  validators.ExactArgs(1),
		Short: ShortBeta("Mint a JWT for a tenant"),
		Long: LongBeta(`Mint a short-lived JWT scoped to a single tenant.

The token grants access to that tenant's data and is valid for 24 hours. Treat it
as a credential: it is intended for your own backend to hand to a tenant's session,
not to be pasted into a shell history or shared.`),
		RunE: tc.runOutpostTenantTokenCmd,
		Example: `  # Mint a token for a tenant
  hookdeck outpost tenant token acme

  # As JSON, for piping into another tool
  hookdeck outpost tenant token acme --output json`,
		Annotations: map[string]string{
			"cli.arguments": `[
				{"name":"tenant-id","type":"string","description":"The ID of the tenant to mint a token for.","required":true}
			]`,
		},
	}

	tc.cmd.Flags().StringVar(&tc.output, "output", "", "Output format (json)")

	return tc
}

func (tc *outpostTenantTokenCmd) runOutpostTenantTokenCmd(cmd *cobra.Command, args []string) error {
	client := Config.GetOutpostAPIClient()

	token, err := client.GetOutpostTenantToken(context.Background(), args[0])
	if err != nil {
		return fmt.Errorf("failed to get tenant token: %w", err)
	}

	if tc.output == "json" {
		return printJSONIndented(token)
	}

	// Print the token alone so it can be captured with $(...) without post-processing.
	fmt.Println(token.Token)

	return nil
}

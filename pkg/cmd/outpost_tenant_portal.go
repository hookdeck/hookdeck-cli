package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/open"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type outpostTenantPortalCmd struct {
	cmd *cobra.Command

	theme  string
	open   bool
	output string
}

func newOutpostTenantPortalCmd() *outpostTenantPortalCmd {
	tc := &outpostTenantPortalCmd{}

	tc.cmd = &cobra.Command{
		Use:   "portal <tenant-id>",
		Args:  validators.ExactArgs(1),
		Short: ShortBeta("Get a tenant's portal URL"),
		Long: LongBeta(`Get a redirect URL for a tenant's portal, where they manage their own destinations.

The URL grants access to that tenant's portal session, so treat it as a credential.

This requires a portal custom domain to be configured for the project; see
'hookdeck outpost config custom-domain'.`),
		PreRunE: tc.validateFlags,
		RunE:    tc.runOutpostTenantPortalCmd,
		Example: `  # Print the portal URL
  hookdeck outpost tenant portal acme

  # Open it in a browser
  hookdeck outpost tenant portal acme --open

  # Request the dark theme
  hookdeck outpost tenant portal acme --theme dark`,
		Annotations: map[string]string{
			"cli.arguments": `[
				{"name":"tenant-id","type":"string","description":"The ID of the tenant whose portal URL to fetch.","required":true}
			]`,
		},
	}

	tc.cmd.Flags().StringVar(&tc.theme, "theme", "", "Portal theme (light, dark)")
	tc.cmd.Flags().BoolVar(&tc.open, "open", false, "Open the portal URL in your browser")
	tc.cmd.Flags().StringVar(&tc.output, "output", "", "Output format (json)")

	return tc
}

func (tc *outpostTenantPortalCmd) validateFlags(cmd *cobra.Command, args []string) error {
	if err := rejectEmptyFlags(cmd); err != nil {
		return err
	}
	if tc.theme != "" && tc.theme != "light" && tc.theme != "dark" {
		return fmt.Errorf("--theme must be either light or dark")
	}
	return nil
}

func (tc *outpostTenantPortalCmd) runOutpostTenantPortalCmd(cmd *cobra.Command, args []string) error {
	client := Config.GetOutpostAPIClient()

	portal, err := client.GetOutpostTenantPortalURL(context.Background(), args[0], tc.theme)
	if err != nil {
		// A 404 here is the documented precondition, not a missing tenant: the
		// endpoint answers this way whenever the project has no portal to
		// redirect to. Passing the raw response through would leave the reader
		// looking for a tenant that exists.
		if hookdeck.IsNotFoundError(err) {
			return fmt.Errorf(`this project has no tenant portal, so there is no URL to return.

A portal needs a custom domain, and the change takes a short while to reach the deployment:

  hookdeck outpost config custom-domain set <hostname>
  hookdeck outpost status`)
		}
		return fmt.Errorf("failed to get tenant portal URL: %w", err)
	}

	if tc.output == "json" {
		return printJSONIndented(portal)
	}

	fmt.Println(portal.RedirectURL)

	if tc.open {
		if err := open.Browser(portal.RedirectURL); err != nil {
			// The URL is already on stdout, so this is a degraded success rather
			// than a failure: report it and let the user open it themselves.
			fmt.Printf("Could not open a browser automatically: %v\n", err)
		}
	}

	return nil
}

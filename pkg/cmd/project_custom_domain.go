package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

// A project's custom domains.
//
// Not the same feature as `hookdeck outpost config custom_domain`, which serves
// the Outpost tenant portal from its own route. These two are easy to confuse
// and are not interchangeable.
type projectCustomDomainCmd struct {
	cmd *cobra.Command
}

func newProjectCustomDomainCmd() *projectCustomDomainCmd {
	dc := &projectCustomDomainCmd{}
	dc.cmd = &cobra.Command{
		Use:     "custom-domain",
		Aliases: []string{"custom-domains"},
		Args:    validators.NoArgs,
		Short:   "Manage a project's custom domains [BETA]",
		Long: `List, add and remove the custom domains attached to a project.

This is the project's own domain configuration. The Outpost tenant portal has a
separate custom domain, managed with ` + "`hookdeck outpost config`" + `.`,
	}
	dc.cmd.AddCommand(newCustomDomainListCmd().cmd)
	dc.cmd.AddCommand(newCustomDomainAddCmd().cmd)
	dc.cmd.AddCommand(newCustomDomainRemoveCmd().cmd)
	return dc
}

type customDomainListCmd struct {
	cmd    *cobra.Command
	output string
}

func newCustomDomainListCmd() *customDomainListCmd {
	lc := &customDomainListCmd{}
	lc.cmd = &cobra.Command{
		Use:     "list <project-id>",
		Args:    validators.ExactArgs(1),
		Short:   "List a project's custom domains",
		Example: `$ hookdeck project custom-domain list tm_123`,
		RunE:    lc.run,
	}
	lc.cmd.Flags().StringVar(&lc.output, "output", "", "Output format: json")
	return lc
}

func (lc *customDomainListCmd) run(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	domains, err := Config.GetAPIClient().ListCustomDomains(apiKeyContext(), args[0])
	if err != nil {
		return err
	}

	if lc.output == "json" {
		out, err := json.MarshalIndent(domains, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	}
	if len(domains) == 0 {
		fmt.Println("No custom domains found.")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tHOSTNAME\tSTATUS")
	for _, d := range domains {
		status := d.Status
		if status == "" {
			status = "—"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", d.ID, d.Hostname, status)
	}
	return w.Flush()
}

type customDomainAddCmd struct {
	cmd    *cobra.Command
	output string
}

func newCustomDomainAddCmd() *customDomainAddCmd {
	ac := &customDomainAddCmd{}
	ac.cmd = &cobra.Command{
		Use:     "add <project-id> <hostname>",
		Args:    validators.ExactArgs(2),
		Short:   "Add a custom domain to a project",
		Example: `$ hookdeck project custom-domain add tm_123 hooks.example.com`,
		RunE:    ac.run,
	}
	ac.cmd.Flags().StringVar(&ac.output, "output", "", "Output format: json")
	return ac
}

func (ac *customDomainAddCmd) run(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	domain, err := Config.GetAPIClient().AddCustomDomain(apiKeyContext(), args[0], args[1])
	if err != nil {
		return err
	}

	if ac.output == "json" {
		out, err := json.MarshalIndent(domain, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	}
	fmt.Printf("✔ Added %s to project %s\n", args[1], args[0])
	// The endpoint echoes back only the hostname, so there is no id to print
	// and no verification status to report. Say where to get them rather than
	// leaving a blank field that reads as a failure.
	fmt.Println("Run `hookdeck project custom-domain list` to see its ID and verification status.")
	return nil
}

type customDomainRemoveCmd struct {
	cmd   *cobra.Command
	force bool
}

func newCustomDomainRemoveCmd() *customDomainRemoveCmd {
	rc := &customDomainRemoveCmd{}
	rc.cmd = &cobra.Command{
		Use:     "remove <project-id> <domain-id>",
		Args:    validators.ExactArgs(2),
		Short:   "Remove a custom domain from a project",
		Long:    `Remove a custom domain. Traffic to that hostname stops being served.`,
		Example: `$ hookdeck project custom-domain remove tm_123 dom_456`,
		RunE:    rc.run,
	}
	rc.cmd.Flags().BoolVar(&rc.force, "force", false, "Skip the confirmation prompt")
	return rc
}

func (rc *customDomainRemoveCmd) run(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	if !rc.force {
		ok, err := confirmDestructiveAction(
			fmt.Sprintf("Remove custom domain %s from project %s?", args[1], args[0]),
			"Removal cancelled.", "force")
		if err != nil {
			return err
		}
		if !ok {
			fmt.Println("Removal cancelled.")
			return nil
		}
	}

	if err := Config.GetAPIClient().DeleteCustomDomain(apiKeyContext(), args[0], args[1]); err != nil {
		return err
	}
	fmt.Printf("✔ Removed custom domain %s\n", args[1])
	return nil
}

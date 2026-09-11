package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type outpostCustomDomainCmd struct {
	cmd *cobra.Command
}

func newOutpostCustomDomainCmd() *outpostCustomDomainCmd {
	cc := &outpostCustomDomainCmd{}

	cc.cmd = &cobra.Command{
		Use:   "custom-domain",
		Args:  validators.NoArgs,
		Short: ShortBeta("Manage the tenant portal's custom domain"),
		Long: LongBeta(`Manage the custom hostname that serves your tenants' portal.

A custom domain is required before 'hookdeck outpost tenant portal' can return a
URL. Adding one returns the DNS records to create; the domain starts working
once they have propagated and been verified.`),
	}

	cc.cmd.AddCommand(newOutpostCustomDomainGetCmd().cmd)
	cc.cmd.AddCommand(newOutpostCustomDomainSetCmd().cmd)
	cc.cmd.AddCommand(newOutpostCustomDomainDeleteCmd().cmd)

	return cc
}

type outpostCustomDomainGetCmd struct {
	cmd *cobra.Command

	output string
}

func newOutpostCustomDomainGetCmd() *outpostCustomDomainGetCmd {
	cc := &outpostCustomDomainGetCmd{}

	cc.cmd = &cobra.Command{
		Use:   "get",
		Args:  validators.NoArgs,
		Short: ShortBeta("Show the portal custom domain"),
		Long:  LongBeta(`Show the custom domain configured for the tenant portal, if any.`),
		RunE:  cc.run,
		Example: `  # Show the configured custom domain
  hookdeck outpost config custom-domain get`,
	}

	cc.cmd.Flags().StringVar(&cc.output, "output", "", "Output format (json)")

	return cc
}

func (cc *outpostCustomDomainGetCmd) run(cmd *cobra.Command, args []string) error {
	client := Config.GetOutpostAPIClient()

	domain, err := client.GetOutpostCustomDomain(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get custom domain: %w", err)
	}

	if cc.output == "json" {
		return printJSONIndented(domain)
	}

	if domain.Hostname == "" {
		fmt.Println("No custom domain is configured.")
		fmt.Println("\nAdd one with 'hookdeck outpost config custom-domain set <hostname>'.")
		return nil
	}

	fmt.Printf("\nHostname: %s\n", domain.Hostname)
	if domain.Status != "" {
		fmt.Printf("Status: %s\n", domain.Status)
	}
	printOutpostDomainVerification(domain.Verification)
	fmt.Println()

	return nil
}

type outpostCustomDomainSetCmd struct {
	cmd *cobra.Command

	output string
}

func newOutpostCustomDomainSetCmd() *outpostCustomDomainSetCmd {
	cc := &outpostCustomDomainSetCmd{}

	cc.cmd = &cobra.Command{
		Use:   "set <hostname>",
		Args:  validators.ExactArgs(1),
		Short: ShortBeta("Set the portal custom domain"),
		Long: LongBeta(`Configure a custom hostname for the tenant portal.

The response includes the DNS records to create. The domain is not usable until
they have propagated and been verified.`),
		RunE: cc.run,
		Example: `  # Configure a custom domain
  hookdeck outpost config custom-domain set portal.example.com`,
		Annotations: map[string]string{
			"cli.arguments": `[
				{"name":"hostname","type":"string","description":"The hostname to serve the tenant portal from.","required":true}
			]`,
		},
	}

	cc.cmd.Flags().StringVar(&cc.output, "output", "", "Output format (json)")

	return cc
}

func (cc *outpostCustomDomainSetCmd) run(cmd *cobra.Command, args []string) error {
	client := Config.GetOutpostAPIClient()

	domain, err := client.AddOutpostCustomDomain(context.Background(), args[0])
	if err != nil {
		return fmt.Errorf("failed to set custom domain: %w", err)
	}

	if cc.output == "json" {
		return printJSONIndented(domain)
	}

	fmt.Printf("%s Custom domain %s configured\n", SuccessCheck, args[0])
	printOutpostDomainVerification(domain.Verification)

	return nil
}

type outpostCustomDomainDeleteCmd struct {
	cmd *cobra.Command

	force bool
}

func newOutpostCustomDomainDeleteCmd() *outpostCustomDomainDeleteCmd {
	cc := &outpostCustomDomainDeleteCmd{}

	cc.cmd = &cobra.Command{
		Use:   "delete",
		Args:  validators.NoArgs,
		Short: ShortBeta("Remove the portal custom domain"),
		Long: LongBeta(`Remove the tenant portal's custom domain.

Tenant portal URLs stop working until another domain is configured.`),
		RunE: cc.run,
		Example: `  # Remove the custom domain, with a confirmation prompt
  hookdeck outpost config custom-domain delete

  # Skip the prompt (for scripts and CI)
  hookdeck outpost config custom-domain delete --force`,
	}

	cc.cmd.Flags().BoolVar(&cc.force, "force", false, "Delete without confirmation")

	return cc
}

func (cc *outpostCustomDomainDeleteCmd) run(cmd *cobra.Command, args []string) error {
	if !cc.force {
		proceed, err := confirmDestructiveAction(
			"\nAre you sure you want to remove the portal custom domain? Tenant portal URLs will stop working.",
			"Deletion cancelled.",
			"force",
		)
		if err != nil {
			return err
		}
		if !proceed {
			return nil
		}
	}

	client := Config.GetOutpostAPIClient()
	if err := client.DeleteOutpostCustomDomain(context.Background()); err != nil {
		return fmt.Errorf("failed to delete custom domain: %w", err)
	}

	fmt.Printf("%s Custom domain removed\n", SuccessCheck)

	return nil
}

// printOutpostDomainVerification renders the DNS records that must exist for the
// domain to verify. The shape is provider-defined, so it is printed generically.
func printOutpostDomainVerification(verification []map[string]interface{}) {
	if len(verification) == 0 {
		return
	}

	fmt.Println("\nCreate these DNS records:")
	for _, record := range verification {
		fmt.Println()
		for _, key := range sortedKeys(record) {
			fmt.Printf("  %s: %v\n", key, record[key])
		}
	}
}

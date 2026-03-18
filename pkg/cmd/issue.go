package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type issueCmd struct {
	cmd *cobra.Command
}

func newIssueCmd() *issueCmd {
	ic := &issueCmd{}

	ic.cmd = &cobra.Command{
		Use:     "issue",
		Aliases: []string{"issues"},
		Args:    validators.NoArgs,
		Short:   ShortBeta("Manage your issues"),
		Long: LongBeta(`Manage Hookdeck issues.

Issues are automatically created when delivery failures, transformation errors,
or backpressure conditions are detected. Use these commands to list, inspect,
update the status of, or dismiss issues.`),
	}

	ic.cmd.AddCommand(newIssueListCmd().cmd)
	ic.cmd.AddCommand(newIssueGetCmd().cmd)
	ic.cmd.AddCommand(newIssueUpdateCmd().cmd)
	ic.cmd.AddCommand(newIssueDismissCmd().cmd)
	ic.cmd.AddCommand(newIssueCountCmd().cmd)

	return ic
}

// addIssueCmdTo registers the issue command tree on the given parent.
func addIssueCmdTo(parent *cobra.Command) {
	parent.AddCommand(newIssueCmd().cmd)
}

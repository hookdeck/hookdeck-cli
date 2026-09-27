package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type sourceCmd struct {
	cmd *cobra.Command
}

func newSourceCmd() *sourceCmd {
	sc := &sourceCmd{}

	sc.cmd = &cobra.Command{
		Use:     "source",
		Aliases: []string{"sources"},
		Args:    validators.NoArgs,
		Short:   "Manage your sources",
		Long: `Manage webhook and event sources.

Sources receive incoming webhooks and events. Create sources with a type (e.g. WEBHOOK, STRIPE)
and optional authentication config, then connect them to destinations via connections.`,
	}

	sc.cmd.AddCommand(newSourceListCmd().cmd)
	sc.cmd.AddCommand(newSourceGetCmd().cmd)
	sc.cmd.AddCommand(newSourceCreateCmd().cmd)
	sc.cmd.AddCommand(newSourceUpsertCmd().cmd)
	sc.cmd.AddCommand(newSourceUpdateCmd().cmd)
	sc.cmd.AddCommand(newSourceDeleteCmd().cmd)
	sc.cmd.AddCommand(newSourceEnableCmd().cmd)
	sc.cmd.AddCommand(newSourceDisableCmd().cmd)
	sc.cmd.AddCommand(newSourceCountCmd().cmd)

	return sc
}

// addSourceCmdTo registers the source command tree on the given parent (e.g. gateway or root).
func addSourceCmdTo(parent *cobra.Command) {
	parent.AddCommand(newSourceCmd().cmd)
}

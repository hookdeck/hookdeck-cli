package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type profileCmd struct {
	cmd *cobra.Command
}

func newProfileCmd() *profileCmd {
	lc := &profileCmd{}

	lc.cmd = &cobra.Command{
		Use:   "profile",
		Args:  validators.NoArgs,
		Short: "Manage your profiles",
	}

	lc.cmd.AddCommand(newProfileListCmd().cmd)
	lc.cmd.AddCommand(newProfileUseCmd().cmd)

	return lc
}

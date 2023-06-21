package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/logout"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type logoutCmd struct {
	cmd *cobra.Command
	all bool
}

func newLogoutCmd() *logoutCmd {
	lc := &logoutCmd{}

	lc.cmd = &cobra.Command{
		Use:   "logout",
		Args:  validators.NoArgs,
		Short: "Logout of your Hookdeck account",
		Long:  `Logout of your Hookdeck account to setup the CLI`,
		RunE:  lc.runLogoutCmd,
	}

	return lc
}

func (lc *logoutCmd) runLogoutCmd(cmd *cobra.Command, args []string) error {
	return logout.Logout(&Config)
}

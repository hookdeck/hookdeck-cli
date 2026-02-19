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
		Example: `  $ hookdeck logout
  $ hookdeck logout -a  # clear all projects`,
		RunE: lc.runLogoutCmd,
	}
	lc.cmd.Flags().BoolVarP(&lc.all, "all", "a", false, "Clear credentials for all projects you are currently logged into.")

	return lc
}

func (lc *logoutCmd) runLogoutCmd(cmd *cobra.Command, args []string) error {
	if lc.all {
		return logout.All(&Config)
	}
	return logout.Logout(&Config)
}

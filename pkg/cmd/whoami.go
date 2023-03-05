package cmd

import (
	"fmt"
	"os"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/login"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
	"github.com/spf13/cobra"
)

type whoamiCmd struct {
	cmd         *cobra.Command
	interactive bool
}

func newWhoamiCmd() *whoamiCmd {
	lc := &whoamiCmd{}

	lc.cmd = &cobra.Command{
		Use:   "whoami",
		Args:  validators.NoArgs,
		Short: "Show the logged-in user",
		RunE:  lc.runWhoamiCmd,
	}

	return lc
}

func (lc *whoamiCmd) runWhoamiCmd(cmd *cobra.Command, args []string) error {
	key, err := Config.Profile.GetAPIKey()
	if err != nil {
		return err
	}
	response, err := login.ValidateKey(Config.APIBaseURL, key)
	if err != nil {
		return err
	}

	color := ansi.Color(os.Stdout)

	fmt.Printf(
		"Logged in as %s in workspace %s\n",
		color.Bold(response.UserName),
		color.Bold(response.TeamName),
	)

	return nil
}

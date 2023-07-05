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
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	color := ansi.Color(os.Stdout)

	fmt.Printf("Using profile %s\n", color.Bold(Config.Profile.Name))

	response, err := login.ValidateKey(Config.APIBaseURL, Config.Profile.APIKey, Config.Profile.TeamID)
	if err != nil {
		return err
	}

	fmt.Printf(
		"Logged in as %s in workspace %s\n",
		color.Bold(response.UserName),
		color.Bold(response.TeamName),
	)

	return nil
}

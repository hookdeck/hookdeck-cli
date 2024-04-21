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

	fmt.Printf("\nUsing profile %s (use -p flag to use a different config profile)\n\n", color.Bold(Config.Profile.Name))

	response, err := login.ValidateKey(Config.APIBaseURL, Config.Profile.APIKey, Config.Profile.TeamID)
	if err != nil {
		return err
	}

	fmt.Printf(
		"Logged in as %s (%s) on project %s in organization %s\n",
		color.Bold(response.UserName),
		color.Bold(response.UserEmail),
		color.Bold(response.TeamName),
		color.Bold(response.OrganizationName),
	)

	return nil
}

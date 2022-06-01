package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
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
		RunE: lc.runWhoamiCmd,
	}

	return lc
}

func (lc *whoamiCmd) runWhoamiCmd(cmd *cobra.Command, args []string) error {
	displayName := Config.Profile.GetDisplayName()
	teamName := Config.Profile.GetTeamName()

	color := ansi.Color(os.Stdout)

	fmt.Printf(
		"Logged in as %s in workspace %s\n",
		color.Bold(displayName),
		color.Bold(teamName),
	)

	return nil
}

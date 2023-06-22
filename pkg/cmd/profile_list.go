package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type profileListCmd struct {
	cmd *cobra.Command
}

func newProfileListCmd() *profileListCmd {
	lc := &profileListCmd{}

	lc.cmd = &cobra.Command{
		Use:   "list",
		Args:  validators.NoArgs,
		Short: "List your profiles",
		RunE:  lc.runProfileListCmd,
	}

	return lc
}

func (lc *profileListCmd) runProfileListCmd(cmd *cobra.Command, args []string) error {	
	profiles := Config.ListProfiles()

	color := ansi.Color(os.Stdout)

	// TODO: show more data per profile

	for _, profile := range profiles {
		if profile == Config.Profile.Name {
			fmt.Printf("%s (current)\n", color.Green(profile))
		} else {
			fmt.Printf("%s\n", profile)
		}
	}

	return nil
}

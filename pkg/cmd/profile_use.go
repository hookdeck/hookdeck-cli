package cmd

import (
	"fmt"
	"os"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type profileUseCmd struct {
	cmd *cobra.Command
}

func newProfileUseCmd() *profileUseCmd {
	lc := &profileUseCmd{}

	lc.cmd = &cobra.Command{
		Use:   "use",
		Args:  validators.MaximumNArgs(1),
		Short: "Select an active workspace for upcoming commands",
		RunE:  lc.runProfileUseCmd,
	}

	return lc
}

func (lc *profileUseCmd) runProfileUseCmd(cmd *cobra.Command, args []string) error {	
	profiles := Config.ListProfiles()

	prompt := promptui.Select{
		Label: "Select Profile",
		Items: profiles,
	}

	_, result, err := prompt.Run()
	if err != nil {
		return err
	}

	color := ansi.Color(os.Stdout)
	fmt.Printf("Selecting profile %s\n", color.Green(result))

	Config.Profile.Name = result
	return Config.Profile.UseProfile()
}

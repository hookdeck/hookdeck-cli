package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/project"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type projectListCmd struct {
	cmd *cobra.Command
}

func newProjectListCmd() *projectListCmd {
	lc := &projectListCmd{}

	lc.cmd = &cobra.Command{
		Use:   "list",
		Args:  validators.NoArgs,
		Short: "List your projects",
		RunE:  lc.runProjectListCmd,
	}

	return lc
}

func (lc *projectListCmd) runProjectListCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	projects, err := project.ListProjects(&Config)
	if err != nil {
		return err
	}

	color := ansi.Color(os.Stdout)

	for _, project := range projects {
		if project.Id == Config.Profile.ProjectId {
			fmt.Printf("%s (current)\n", color.Green(project.Name))
		} else {
			fmt.Printf("%s\n", project.Name)
		}
	}

	return nil
}

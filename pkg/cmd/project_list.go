package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/project"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

var validProjectTypes = []string{"gateway", "outpost", "console"}

type projectListCmd struct {
	cmd         *cobra.Command
	output      string
	typeFilter  string
}

func newProjectListCmd() *projectListCmd {
	lc := &projectListCmd{}

	lc.cmd = &cobra.Command{
		Use:     "list [<organization_substring>] [<project_substring>]",
		Args:    validators.MaximumNArgs(2),
		Short:   "List and filter projects by organization and project name substrings",
		RunE:    lc.runProjectListCmd,
		Example: `$ hookdeck project list
Acme / Ecommerce Production (current) | Gateway
Acme / Ecommerce Staging | Gateway
$ hookdeck project list --output json
$ hookdeck project list --type gateway`,
	}

	lc.cmd.Flags().StringVar(&lc.output, "output", "", "Output format: json")
	lc.cmd.Flags().StringVar(&lc.typeFilter, "type", "", "Filter by project type: gateway, outpost, console")

	return lc
}

func (lc *projectListCmd) runProjectListCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	if lc.typeFilter != "" {
		ok := false
		for _, v := range validProjectTypes {
			if lc.typeFilter == v {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("invalid --type value: %q (must be one of: gateway, outpost, console)", lc.typeFilter)
		}
	}

	projects, err := project.ListProjects(&Config)
	if err != nil {
		return err
	}

	items := project.NormalizeProjects(projects, Config.Profile.ProjectId)
	items = project.FilterByType(items, lc.typeFilter)

	switch len(args) {
	case 1:
		items = project.FilterByOrgProject(items, args[0], "")
	case 2:
		items = project.FilterByOrgProject(items, args[0], args[1])
	}

	if len(items) == 0 {
		if lc.output == "json" {
			fmt.Println("[]")
			return nil
		}
		fmt.Println("No projects found.")
		return nil
	}

	if lc.output == "json" {
		type jsonItem struct {
			Id      string `json:"id"`
			Org     string `json:"org"`
			Project string `json:"project"`
			Type    string `json:"type"`
			Current bool   `json:"current"`
		}
		out := make([]jsonItem, len(items))
		for i, it := range items {
			out[i] = jsonItem{
				Id:      it.Id,
				Org:     it.Org,
				Project: it.Project,
				Type:    config.ProjectTypeToJSON(it.Type),
				Current: it.Current,
			}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	color := ansi.Color(os.Stdout)
	for _, it := range items {
		if it.Current {
			// highlight (current) in green
			namePart := it.Project
			if it.Org != "" {
				namePart = it.Org + " / " + it.Project
			}
			fmt.Printf("%s%s | %s\n", namePart, color.Green(" (current)"), it.Type)
		} else {
			fmt.Println(it.DisplayLine())
		}
	}

	return nil
}

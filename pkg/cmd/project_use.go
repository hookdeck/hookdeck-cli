package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/project"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type projectUseCmd struct {
	cmd   *cobra.Command
	local bool
}

func newProjectUseCmd() *projectUseCmd {
	lc := &projectUseCmd{}

	lc.cmd = &cobra.Command{
		Use:     "use [<organization_name> [<project_name>]]",
		Args:    validators.MaximumNArgs(2),
		Short:   "Set the active project for future commands",
		RunE:    lc.runProjectUseCmd,
		Example: `$ hookdeck project use
Use the arrow keys to navigate: ↓ ↑ → ←
? Select Project:
  ▸ Acme / Ecommerce Production (current) | Gateway
    Acme / Ecommerce Staging | Gateway

$ hookdeck project use --local
Pinning project to current directory`,
	}

	lc.cmd.Flags().BoolVar(&lc.local, "local", false, "Save project to current directory (.hookdeck/config.toml)")

	return lc
}

func (lc *projectUseCmd) runProjectUseCmd(cmd *cobra.Command, args []string) error {
	if lc.local && Config.ConfigFileFlag != "" {
		return fmt.Errorf("Error: --local and --hookdeck-config flags cannot be used together\n  --local creates config at: .hookdeck/config.toml\n  --hookdeck-config uses custom path: %s", Config.ConfigFileFlag)
	}

	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	projects, err := project.ListProjects(&Config)
	if err != nil {
		return err
	}

	items := project.NormalizeProjects(projects, Config.Profile.ProjectId)
	if len(items) == 0 {
		return fmt.Errorf("no projects found. Please create a project first using 'hookdeck project create'")
	}

	// Filter by exact org and/or project when args provided
	switch len(args) {
	case 1:
		items = filterItemsByExactOrg(items, args[0])
		if len(items) == 0 {
			return fmt.Errorf("no projects found for organization '%s'", args[0])
		}
	case 2:
		items = filterItemsByExactOrgProject(items, args[0], args[1])
		if len(items) == 0 {
			return fmt.Errorf("project '%s' in organization '%s' not found", args[1], args[0])
		}
		if len(items) > 1 {
			return fmt.Errorf("multiple projects named '%s' found in organization '%s'. Projects must have unique names to be used with the `project use <org> <project>` command", args[1], args[0])
		}
	}

	var selected *project.ProjectListItem
	if len(args) == 2 || len(items) == 1 {
		selected = &items[0]
	} else {
		options := make([]string, len(items))
		var defaultOpt string
		for i, it := range items {
			options[i] = it.DisplayLine()
			if it.Current {
				defaultOpt = options[i]
			}
		}
		message := "Select Project"
		if len(args) == 1 {
			message = fmt.Sprintf("Select project for organization '%s'", args[0])
		}
		prompt := &survey.Select{
			Message: message,
			Options: options,
			Default: defaultOpt,
		}
		var selectedOption string
		if err := survey.AskOne(prompt, &selectedOption); err != nil {
			return err
		}
		for i := range options {
			if options[i] == selectedOption {
				selected = &items[i]
				break
			}
		}
		if selected == nil {
			return fmt.Errorf("internal error: selected project not found in list")
		}
	}

	// Use project by id and mode derived from type
	mode := config.ProjectTypeToMode(selected.Type)
	var configPath string
	var isNewConfig bool

	if lc.local {
		isNewConfig, err = Config.UseProjectLocal(selected.Id, mode)
		if err != nil {
			return err
		}
		workingDir, wdErr := os.Getwd()
		if wdErr != nil {
			return wdErr
		}
		configPath = filepath.Join(workingDir, ".hookdeck/config.toml")
	} else {
		workingDir, wdErr := os.Getwd()
		if wdErr != nil {
			return wdErr
		}
		localConfigPath := filepath.Join(workingDir, ".hookdeck/config.toml")
		localConfigExists, _ := Config.FileExists(localConfigPath)

		if localConfigExists {
			isNewConfig, err = Config.UseProjectLocal(selected.Id, mode)
			if err != nil {
				return err
			}
			configPath = localConfigPath
		} else {
			err = Config.UseProject(selected.Id, mode)
			if err != nil {
				return err
			}
			configPath = Config.GetConfigFile()
			isNewConfig = false
		}
	}

	displayName := selected.Project
	if selected.Org != "" {
		displayName = selected.Org + " / " + selected.Project
	}
	color := ansi.Color(os.Stdout)
	fmt.Printf("Successfully set active project to: %s\n", color.Green(displayName))

	if strings.Contains(configPath, ".hookdeck/config.toml") {
		if isNewConfig && lc.local {
			fmt.Printf("Created: %s\n", configPath)
			fmt.Printf("\n%s\n", color.Yellow("Security:"))
			fmt.Printf("  Local config files contain credentials and should NOT be committed to source control.\n")
			fmt.Printf("  Add .hookdeck/ to your .gitignore file.\n")
		} else {
			fmt.Printf("Updated: %s\n", configPath)
		}
	} else {
		fmt.Printf("Saved to: %s\n", configPath)
	}

	return nil
}

func filterItemsByExactOrg(items []project.ProjectListItem, org string) []project.ProjectListItem {
	orgLower := strings.ToLower(org)
	var out []project.ProjectListItem
	for _, it := range items {
		if strings.ToLower(it.Org) == orgLower {
			out = append(out, it)
		}
	}
	return out
}

func filterItemsByExactOrgProject(items []project.ProjectListItem, org, proj string) []project.ProjectListItem {
	orgLower := strings.ToLower(org)
	projLower := strings.ToLower(proj)
	var out []project.ProjectListItem
	for _, it := range items {
		if strings.ToLower(it.Org) == orgLower && strings.ToLower(it.Project) == projLower {
			out = append(out, it)
		}
	}
	return out
}

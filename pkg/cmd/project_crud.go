package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

func renderProject(p *hookdeck.Project, output string) error {
	if output == "json" {
		out, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	}
	fmt.Printf("Project: %s\n", p.Name)
	fmt.Printf("ID:      %s\n", p.Id)
	fmt.Printf("Type:    %s\n", config.ProjectTypeToJSON(p.Type))
	return nil
}

// --- get ---

type projectGetCmd struct {
	cmd    *cobra.Command
	output string
}

func newProjectGetCmd() *projectGetCmd {
	gc := &projectGetCmd{}
	gc.cmd = &cobra.Command{
		Use:     "get <id>",
		Args:    validators.ExactArgs(1),
		Short:   "Show a project",
		Long:    `Show one project by ID.`,
		Example: `$ hookdeck project get tm_123`,
		RunE:    gc.run,
	}
	gc.cmd.Flags().StringVar(&gc.output, "output", "", "Output format: json")
	return gc
}

func (gc *projectGetCmd) run(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	p, err := Config.GetAPIClient().GetProject(apiKeyContext(), args[0])
	if err != nil {
		return err
	}
	return renderProject(p, gc.output)
}

// --- create ---

type projectCreateCmd struct {
	cmd         *cobra.Command
	name        string
	projectType string
	private     bool
	output      string
}

func newProjectCreateCmd() *projectCreateCmd {
	cc := &projectCreateCmd{}
	cc.cmd = &cobra.Command{
		Use:   "create",
		Args:  validators.NoArgs,
		Short: "Create a project",
		Long:  `Create a project in your organization.`,
		Example: `$ hookdeck project create --name Staging --type gateway
$ hookdeck project create --name "Outpost staging" --type outpost --private`,
		RunE: cc.run,
	}
	cc.cmd.Flags().StringVar(&cc.name, "name", "", "Project name (required)")
	cc.cmd.Flags().StringVar(&cc.projectType, "type", "", "Project type: gateway or outpost (required)")
	cc.cmd.Flags().BoolVar(&cc.private, "private", false, "Create the project as private")
	cc.cmd.Flags().StringVar(&cc.output, "output", "", "Output format: json")
	return cc
}

func (cc *projectCreateCmd) run(cmd *cobra.Command, args []string) error {
	// Argument validation first: a malformed command should not need
	// credentials to be told it is malformed.
	//
	// The endpoint declares no required fields, so a bare POST is valid and
	// makes an unnamed project of unspecified type. Nobody means that.
	if cc.name == "" {
		return fmt.Errorf("--name is required")
	}
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	if cc.projectType == "" {
		return fmt.Errorf("--type is required: gateway or outpost")
	}
	apiType := config.NormalizeProjectType(cc.projectType)
	if apiType != config.ProjectTypeEventGateway && apiType != config.ProjectTypeOutpost {
		return fmt.Errorf("invalid --type value: %q (must be gateway or outpost)", cc.projectType)
	}

	req := &hookdeck.ProjectCreateRequest{Name: &cc.name, Type: &apiType}
	if cmd.Flags().Changed("private") {
		req.Private = &cc.private
	}

	p, err := Config.GetAPIClient().CreateProject(apiKeyContext(), req)
	if err != nil {
		return err
	}
	return renderProject(p, cc.output)
}

// --- update ---

type projectUpdateCmd struct {
	cmd           *cobra.Command
	name          string
	domain        string
	headersPrefix string
	private       bool
	output        string
}

func newProjectUpdateCmd() *projectUpdateCmd {
	uc := &projectUpdateCmd{}
	uc.cmd = &cobra.Command{
		Use:     "update <id>",
		Args:    validators.ExactArgs(1),
		Short:   "Update a project",
		Long:    `Change a project's name or settings.`,
		Example: `$ hookdeck project update tm_123 --name "Staging"`,
		RunE:    uc.run,
	}
	uc.cmd.Flags().StringVar(&uc.name, "name", "", "New project name")
	uc.cmd.Flags().StringVar(&uc.domain, "domain", "", "Project domain")
	uc.cmd.Flags().StringVar(&uc.headersPrefix, "headers-prefix", "", "Prefix applied to forwarded headers")
	uc.cmd.Flags().BoolVar(&uc.private, "private", false, "Whether the project is private")
	uc.cmd.Flags().StringVar(&uc.output, "output", "", "Output format: json")
	return uc
}

func (uc *projectUpdateCmd) run(cmd *cobra.Command, args []string) error {
	req := &hookdeck.ProjectUpdateRequest{}
	if cmd.Flags().Changed("name") {
		req.Name = &uc.name
	}
	if cmd.Flags().Changed("domain") {
		req.Domain = &uc.domain
	}
	if cmd.Flags().Changed("headers-prefix") {
		req.HeadersPrefix = &uc.headersPrefix
	}
	if cmd.Flags().Changed("private") {
		req.Private = &uc.private
	}
	if req.Name == nil && req.Domain == nil && req.HeadersPrefix == nil && req.Private == nil {
		// An empty PUT succeeds and changes nothing, which would exit 0 for a
		// command that did nothing.
		return fmt.Errorf("nothing to update: pass --name, --domain, --headers-prefix or --private")
	}
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	p, err := Config.GetAPIClient().UpdateProject(apiKeyContext(), args[0], req)
	if err != nil {
		return err
	}
	return renderProject(p, uc.output)
}

// --- delete ---

type projectDeleteCmd struct {
	cmd   *cobra.Command
	force bool
}

func newProjectDeleteCmd() *projectDeleteCmd {
	dc := &projectDeleteCmd{}
	dc.cmd = &cobra.Command{
		Use:   "delete <id>",
		Args:  validators.ExactArgs(1),
		Short: "Delete a project",
		Long: `Delete a project. Everything in it — connections, sources, destinations, events and
their history — goes with it, and none of it can be recovered.`,
		Example: `$ hookdeck project delete tm_123
$ hookdeck project delete tm_123 --force`,
		RunE: dc.run,
	}
	dc.cmd.Flags().BoolVar(&dc.force, "force", false, "Skip the confirmation prompt")
	return dc
}

func (dc *projectDeleteCmd) run(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	if !dc.force {
		ok, err := confirmDestructiveAction(
			fmt.Sprintf("Delete project %s and everything in it?", args[0]),
			"Deletion cancelled.", "force")
		if err != nil {
			return err
		}
		if !ok {
			fmt.Println("Deletion cancelled.")
			return nil
		}
	}

	if err := Config.GetAPIClient().DeleteProject(apiKeyContext(), args[0]); err != nil {
		return err
	}
	fmt.Printf("✔ Project %s deleted\n", args[0])
	return nil
}

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/project"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type outpostCmd struct {
	cmd *cobra.Command
}

// isOutpostMCPLeafCommand reports whether cmd is the outpost mcp subcommand.
// MCP speaks JSON-RPC on stdout, so when there is no API key yet the project
// check must not run — the server needs to start and expose its login tool.
func isOutpostMCPLeafCommand(cmd *cobra.Command) bool {
	return cmd != nil && cmd.Name() == "mcp" && cmd.Parent() != nil && cmd.Parent().Name() == "outpost"
}

// outpostPersistentPreRunE runs before every outpost subcommand. Cobra does not
// chain PersistentPreRun, so initTelemetry must be called here explicitly.
func outpostPersistentPreRunE(cmd *cobra.Command, args []string) error {
	initTelemetry(cmd)
	if isOutpostMCPLeafCommand(cmd) {
		if err := Config.Profile.ValidateAPIKey(); err != nil {
			return nil
		}
	}
	return requireOutpostProject(nil)
}

// requireOutpostProject ensures the active project is an Outpost project.
//
// Without this the API answers 404 for a Gateway project, which reads as "the
// resource does not exist" rather than "you are pointed at the wrong project".
// cfg is optional; when nil the global Config is used.
func requireOutpostProject(cfg *config.Config) error {
	if cfg == nil {
		cfg = &Config
	}
	if err := cfg.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	if cfg.Profile.ProjectId == "" {
		return fmt.Errorf("no project selected. Run 'hookdeck project use' to select a project")
	}

	projectType := cfg.Profile.ProjectType
	if projectType == "" && cfg.Profile.ProjectMode != "" {
		projectType = config.ModeToProjectType(cfg.Profile.ProjectMode)
	}
	if projectType == "" {
		// Resolve from the API, which is authoritative for the key.
		response, err := cfg.GetAPIClient().ValidateAPIKey()
		if err != nil {
			return err
		}
		cfg.Profile.ApplyValidateAPIKeyResponse(response, false)
		projectType = cfg.Profile.ProjectType
		_ = cfg.Profile.SaveProfile()
	}

	if !config.IsOutpostProject(projectType) {
		return wrongProjectTypeError(cfg, projectType)
	}
	return nil
}

// wrongProjectTypeError explains how to reach an Outpost project from here.
//
// "Run 'hookdeck project use'" is the right advice only when the stored
// credential can switch projects. A project-scoped key cannot, and `project use`
// answers it by saying to log in again — which, with the same key, lands back on
// this error. Naming the real cause is what breaks that circle.
//
// Establishing the credential's scope costs a request, so it is only made here,
// on a path that has already failed. If the check itself fails, fall back to the
// general advice rather than compounding one error with another.
func wrongProjectTypeError(cfg *config.Config, projectType string) error {
	described := projectType
	if described == "" {
		described = "unknown"
	}

	scoped, err := project.CredentialsLackUserAssociation(cfg.GetAPIClient())
	if err == nil && scoped {
		return fmt.Errorf(`this command requires an Outpost project; the active project is a %s project.

The stored credential belongs to that one project, so 'hookdeck project use' cannot switch away from it. Either sign in with an account-wide key:

  hookdeck login

or point this machine at the Outpost project directly, using that project's API key:

  hookdeck ci --api-key <outpost project api key>`, described)
	}

	return fmt.Errorf("this command requires an Outpost project; current project type is %s. Use 'hookdeck project use' to switch to an Outpost project", described)
}

func newOutpostCmd() *outpostCmd {
	oc := &outpostCmd{}

	oc.cmd = &cobra.Command{
		Use:   "outpost",
		Args:  validators.NoArgs,
		Short: ShortBeta("Manage your Hookdeck Outpost resources"),
		Long: LongBeta(`Commands for managing Hookdeck Outpost tenants, destinations, events,
attempts, topics, metrics, and project configuration.

Outpost delivers events to your users' destinations. Each of your users is a tenant,
and each tenant owns the destinations their events are delivered to.

These commands require an Outpost project. Use 'hookdeck project use' to switch.`),
		Example: `  # List tenants
  hookdeck outpost tenant list

  # Create a webhook destination for a tenant
  hookdeck outpost destination create --tenant-id acme --type webhook --config url=https://example.com/hooks

  # Inspect recent events
  hookdeck outpost event list --limit 10

  # Check the deployment status
  hookdeck outpost status`,
		PersistentPreRunE: outpostPersistentPreRunE,
	}

	oc.cmd.AddCommand(newOutpostTenantCmd().cmd)
	oc.cmd.AddCommand(newOutpostDestinationCmd().cmd)
	oc.cmd.AddCommand(newOutpostDestinationTypeCmd().cmd)
	oc.cmd.AddCommand(newOutpostEventCmd().cmd)
	oc.cmd.AddCommand(newOutpostAttemptCmd().cmd)
	oc.cmd.AddCommand(newOutpostTopicCmd().cmd)
	oc.cmd.AddCommand(newOutpostStatusCmd().cmd)
	oc.cmd.AddCommand(newOutpostPublishCmd().cmd)
	oc.cmd.AddCommand(newOutpostMetricsCmd().cmd)
	oc.cmd.AddCommand(newOutpostConfigCmd().cmd)
	addOutpostMCPCmdTo(oc.cmd)

	return oc
}

// addOutpostCmdTo registers the outpost command tree on the given parent.
func addOutpostCmdTo(parent *cobra.Command) {
	parent.AddCommand(newOutpostCmd().cmd)
}

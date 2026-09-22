package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

// --- list ---

type apiKeyListCmd struct {
	cmd    *cobra.Command
	output string
}

func newAPIKeyListCmd() *apiKeyListCmd {
	lc := &apiKeyListCmd{}
	lc.cmd = &cobra.Command{
		Use:   "list",
		Args:  validators.NoArgs,
		Short: "List the organization's API keys",
		Long:  `List the API keys of the current organization, both organization-scoped and project-scoped. Secrets are never shown; keys are identified by fingerprint.`,
		Example: `$ hookdeck org api-key list
$ hookdeck org api-key list --output json`,
		RunE: lc.run,
	}
	lc.cmd.Flags().StringVar(&lc.output, "output", "", "Output format: json")
	return lc
}

func (lc *apiKeyListCmd) run(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	keys, err := Config.GetAPIClient().ListAPIKeys(apiKeyContext())
	if err != nil {
		return orgAuthError(err)
	}
	return renderKeys(keys, lc.output)
}

// --- create ---

type apiKeyCreateCmd struct {
	cmd     *cobra.Command
	label   string
	project string
	scopes  []string
	output  string
}

func newAPIKeyCreateCmd() *apiKeyCreateCmd {
	cc := &apiKeyCreateCmd{}
	cc.cmd = &cobra.Command{
		Use:   "create",
		Args:  validators.NoArgs,
		Short: "Create an API key",
		Long: `Create an API key in the current organization.

Without --project the key is organization-scoped. With it, the key is scoped to
that one project. The secret is shown once, here, and never again.`,
		Example: `$ hookdeck org api-key create --label "CI pipeline"
$ hookdeck org api-key create --label "CI" --project production
$ hookdeck org api-key create --label "read only" --scope gateway.events.read --scope gateway.sources.read`,
		RunE: cc.run,
	}
	cc.cmd.Flags().StringVar(&cc.label, "label", "", "Label for the key (required)")
	cc.cmd.Flags().StringVar(&cc.project, "project", "", "Scope the key to one project, by ID or name. Omit for an organization key")
	cc.cmd.Flags().StringArrayVar(&cc.scopes, "scope", nil, scopeFlagUsage)
	cc.cmd.Flags().StringVar(&cc.output, "output", "", "Output format: json")
	return cc
}

func (cc *apiKeyCreateCmd) run(cmd *cobra.Command, args []string) error {
	if cc.label == "" {
		return fmt.Errorf("--label is required")
	}
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	req := &hookdeck.APIKeyCreateRequest{Label: cc.label, Type: "organization", Scopes: cc.scopes}
	if cc.project != "" {
		id, err := resolveProjectID(cc.project)
		if err != nil {
			return err
		}
		req.Type = "project"
		req.TeamID = &id
	}

	key, err := Config.GetAPIClient().CreateAPIKey(apiKeyContext(), req)
	if err != nil {
		return orgAuthError(err)
	}
	return printSecretOnce(key, cc.output)
}

// --- update ---

type apiKeyUpdateCmd struct {
	cmd    *cobra.Command
	scopes []string
	output string
}

func newAPIKeyUpdateCmd() *apiKeyUpdateCmd {
	uc := &apiKeyUpdateCmd{}
	uc.cmd = &cobra.Command{
		Use:   "update <id>",
		Args:  validators.ExactArgs(1),
		Short: "Change an API key's scopes",
		Long:  `Change the permission scopes of an existing API key. The secret is unchanged.`,
		Example: `$ hookdeck org api-key update apk_123 --scope gateway.events.read
$ hookdeck org api-key update apk_123 --scope "*"`,
		RunE: uc.run,
	}
	uc.cmd.Flags().StringArrayVar(&uc.scopes, "scope", nil, scopeFlagUsage)
	uc.cmd.Flags().StringVar(&uc.output, "output", "", "Output format: json")
	return uc
}

func (uc *apiKeyUpdateCmd) run(cmd *cobra.Command, args []string) error {
	if len(uc.scopes) == 0 {
		// An empty PUT succeeds and changes nothing, which would report success
		// for a command that did not do anything.
		return fmt.Errorf("--scope is required: pass the scopes the key should have")
	}
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	key, err := Config.GetAPIClient().UpdateAPIKey(apiKeyContext(), args[0],
		&hookdeck.APIKeyUpdateRequest{Scopes: uc.scopes})
	if err != nil {
		return err
	}
	// The secret is unchanged, so it must not be reprinted here.
	return renderKeys([]hookdeck.APIKey{*key}, uc.output)
}

// --- roll ---

type apiKeyRollCmd struct {
	cmd      *cobra.Command
	delaySec int
	output   string
}

func newAPIKeyRollCmd() *apiKeyRollCmd {
	rc := &apiKeyRollCmd{}
	rc.cmd = &cobra.Command{
		Use:   "roll <id>",
		Args:  validators.ExactArgs(1),
		Short: "Roll an API key",
		Long: `Replace an API key with a new one. The old key keeps working for --delay seconds,
so callers using it have a window to pick up the replacement.

The new secret is shown once, here.`,
		Example: `$ hookdeck org api-key roll apk_123 --delay 3600`,
		RunE:    rc.run,
	}
	rc.cmd.Flags().IntVar(&rc.delaySec, "delay", 0, "Seconds before the old key stops working (required)")
	rc.cmd.Flags().StringVar(&rc.output, "output", "", "Output format: json")
	return rc
}

func (rc *apiKeyRollCmd) run(cmd *cobra.Command, args []string) error {
	if !cmd.Flags().Changed("delay") {
		// The API requires delay_sec. Defaulting it silently would pick an
		// expiry window on the caller's behalf for a credential they rely on.
		return fmt.Errorf("--delay is required: the number of seconds the old key keeps working")
	}
	if rc.delaySec < 0 {
		return fmt.Errorf("--delay cannot be negative")
	}
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	key, err := Config.GetAPIClient().RollAPIKey(apiKeyContext(), args[0], rc.delaySec)
	if err != nil {
		return orgAuthError(err)
	}
	return printSecretOnce(key, rc.output)
}

// --- delete ---

type apiKeyDeleteCmd struct {
	cmd   *cobra.Command
	force bool
}

func newAPIKeyDeleteCmd() *apiKeyDeleteCmd {
	dc := &apiKeyDeleteCmd{}
	dc.cmd = &cobra.Command{
		Use:   "delete <id>",
		Args:  validators.ExactArgs(1),
		Short: "Delete an API key",
		Long: `Delete an API key. It stops authenticating immediately — anything using it starts
failing at once. To rotate a key with a grace period, use roll instead.`,
		Example: `$ hookdeck org api-key delete apk_123
$ hookdeck org api-key delete apk_123 --force`,
		RunE: dc.run,
	}
	dc.cmd.Flags().BoolVar(&dc.force, "force", false, "Skip the confirmation prompt")
	return dc
}

func (dc *apiKeyDeleteCmd) run(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	if !dc.force {
		ok, err := confirmDestructiveAction(
			fmt.Sprintf("Delete API key %s? It stops authenticating immediately.", args[0]),
			"Deletion cancelled.", "force")
		if err != nil {
			return err
		}
		if !ok {
			fmt.Println("Deletion cancelled.")
			return nil
		}
	}

	if err := Config.GetAPIClient().DeleteAPIKey(apiKeyContext(), args[0]); err != nil {
		return orgAuthError(err)
	}
	fmt.Printf("✔ API key %s deleted\n", args[0])
	return nil
}

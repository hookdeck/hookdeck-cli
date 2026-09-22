package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/project"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

// One command family covers both kinds of key, because one endpoint does:
// POST /organizations/current/api-keys takes type organization or project, and
// the listing returns both. A `hookdeck project api-key` would have been the
// obvious shape and cannot express an organization key at all.
type apiKeyCmd struct {
	cmd *cobra.Command
}

func newAPIKeyCmd() *apiKeyCmd {
	kc := &apiKeyCmd{}

	kc.cmd = &cobra.Command{
		Use:     "api-key",
		Aliases: []string{"api-keys"},
		Args:    validators.NoArgs,
		Short:   "Manage your organization's API keys [BETA]",
		Long: `Create, inspect, roll and delete the API keys of the current organization.

A key is either organization-scoped or scoped to one project; pass --project to
create the latter. Both kinds are listed together, because the API stores them
together.

Secrets are shown once, when a key is created or rolled. Everywhere else a key
is identified by its fingerprint.`,
	}

	kc.cmd.AddCommand(newAPIKeyListCmd().cmd)
	kc.cmd.AddCommand(newAPIKeyCreateCmd().cmd)
	kc.cmd.AddCommand(newAPIKeyUpdateCmd().cmd)
	kc.cmd.AddCommand(newAPIKeyRollCmd().cmd)
	kc.cmd.AddCommand(newAPIKeyDeleteCmd().cmd)

	return kc
}

// scopeFlagUsage is shared so every command says the same thing about scopes.
//
// The OpenAPI document declares no enum for them — only examples of the shape.
// There is nothing to validate against locally, and hardcoding a list would
// drift silently the first time a scope is added.
const scopeFlagUsage = "Permission scope; repeat for several. Defaults to full access. " +
	"The API is the authority on valid scopes — they are passed through unchecked"

// renderKeys prints keys without their secrets.
//
// list returns the `key` field like every other endpoint. Printing it by habit
// would put live credentials into a terminal, a scrollback buffer, and any CI
// log that captured the output.
func renderKeys(keys []hookdeck.APIKey, output string) error {
	if output == "json" {
		safe := make([]hookdeck.APIKey, len(keys))
		for i, k := range keys {
			k.Key = ""
			safe[i] = k
		}
		out, err := json.MarshalIndent(safe, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	}

	if len(keys) == 0 {
		fmt.Println("No API keys found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tLABEL\tSCOPE\tFINGERPRINT\tSCOPES")
	for _, k := range keys {
		scope := "organization"
		if k.TeamID != nil && *k.TeamID != "" {
			scope = "project " + *k.TeamID
		}
		fingerprint := "—"
		if k.KeyFingerprint != nil {
			fingerprint = *k.KeyFingerprint
		}
		scopes := "*"
		if len(k.Scopes) > 0 {
			scopes = strings.Join(k.Scopes, ",")
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", k.ID, k.Label, scope, fingerprint, scopes)
	}
	return w.Flush()
}

// printSecretOnce renders a newly issued key. This is the only place the secret
// is ever printed, and the only chance the caller gets to copy it.
func printSecretOnce(key *hookdeck.APIKey, output string) error {
	if output == "json" {
		out, err := json.MarshalIndent(key, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	}

	fmt.Printf("✔ API key %s created\n\n", key.Label)
	fmt.Printf("  ID:  %s\n", key.ID)
	fmt.Printf("  Key: %s\n\n", key.Key)
	fmt.Println("This is the only time the key is shown. Store it somewhere safe now.")
	return nil
}

// resolveProjectID accepts an id or a name, reusing the resolver `project use`
// already has so --project prod works as well as --project tm_9bML1qH2vXp3.
func resolveProjectID(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if err := project.EnsureUserAssociatedCredentials(&Config); err != nil {
		return "", err
	}
	projects, err := project.ListProjects(&Config)
	if err != nil {
		return "", err
	}
	items := project.NormalizeProjects(projects, Config.Profile.ProjectId)

	for _, it := range items {
		if it.Id == value {
			return it.Id, nil
		}
	}
	var matches []string
	var found string
	for _, it := range items {
		if strings.EqualFold(it.Project, value) {
			matches = append(matches, it.Id)
			found = it.Id
		}
	}
	switch len(matches) {
	case 1:
		return found, nil
	case 0:
		return "", fmt.Errorf("no project matching %q; run `hookdeck project list` to see them", value)
	default:
		// Project names are unique per organization, not per account, so a name
		// can legitimately match twice.
		return "", fmt.Errorf("%q matches %d projects (%s); pass the project ID instead",
			value, len(matches), strings.Join(matches, ", "))
	}
}

func apiKeyContext() context.Context { return context.Background() }

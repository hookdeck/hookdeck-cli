package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type outpostConfigCmd struct {
	cmd *cobra.Command
}

func newOutpostConfigCmd() *outpostConfigCmd {
	cc := &outpostConfigCmd{}

	cc.cmd = &cobra.Command{
		Use:     "config",
		Aliases: []string{"configs"},
		Args:    validators.NoArgs,
		Short:   ShortBeta("Manage Outpost project configuration"),
		Long: LongBeta(`Read and change this project's Outpost configuration.

These settings apply to the whole project — every tenant and destination — so a
change here affects all delivery. Changes take a short while to reach the
deployment; 'hookdeck outpost status' reports when it is still being applied.`),
	}

	cc.cmd.AddCommand(newOutpostConfigGetCmd().cmd)
	cc.cmd.AddCommand(newOutpostConfigSetCmd().cmd)
	cc.cmd.AddCommand(newOutpostCustomDomainCmd().cmd)

	return cc
}

type outpostConfigGetCmd struct {
	cmd *cobra.Command

	output string
}

func newOutpostConfigGetCmd() *outpostConfigGetCmd {
	cc := &outpostConfigGetCmd{}

	cc.cmd = &cobra.Command{
		Use:   "get [key]",
		Args:  validators.MaximumNArgs(1),
		Short: ShortBeta("Show project configuration"),
		Long: LongBeta(`Show this project's Outpost configuration.

Pass a key to print just that value, which is convenient in scripts. Unset keys
are omitted unless you ask for one by name.`),
		RunE: cc.run,
		Example: `  # Show everything that is set
  hookdeck outpost config get

  # Show one value
  hookdeck outpost config get TOPICS`,
		Annotations: map[string]string{
			"cli.arguments": `[
				{"name":"key","type":"string","description":"A single configuration key to print.","required":false}
			]`,
		},
	}

	cc.cmd.Flags().StringVar(&cc.output, "output", "", "Output format (json)")

	return cc
}

func (cc *outpostConfigGetCmd) run(cmd *cobra.Command, args []string) error {
	client := Config.GetOutpostAPIClient()

	config, err := client.GetOutpostConfig(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	if len(args) == 1 {
		value, present := config[args[0]]
		if !present {
			return fmt.Errorf("no configuration key named %q", args[0])
		}
		if cc.output == "json" {
			return printJSONIndented(map[string]*string{args[0]: value})
		}
		if value != nil {
			fmt.Println(*value)
		}
		return nil
	}

	if cc.output == "json" {
		return printJSONIndented(config)
	}

	keys := make([]string, 0, len(config))
	for key, value := range config {
		if value != nil && *value != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	if len(keys) == 0 {
		fmt.Println("No configuration values are set; the deployment is using its defaults.")
		return nil
	}

	color := ansi.Color(os.Stdout)
	fmt.Printf("\n%d configuration value(s) set:\n\n", len(keys))
	for _, key := range keys {
		fmt.Printf("  %s = %s\n", color.Green(key), *config[key])
	}
	fmt.Println()

	return nil
}

type outpostConfigSetCmd struct {
	cmd *cobra.Command

	unset      []string
	configFile string
	dryRun     bool
	output     string
}

func newOutpostConfigSetCmd() *outpostConfigSetCmd {
	cc := &outpostConfigSetCmd{}

	cc.cmd = &cobra.Command{
		Use:   "set [KEY=VALUE ...]",
		Short: ShortBeta("Change project configuration"),
		Long: LongBeta(`Change this project's Outpost configuration.

Only the keys you pass are changed. --unset returns a key to its default.

This affects delivery for every tenant in the project, so use --dry-run first to
see exactly what would change.

Some keys are managed for you and are rejected if set directly; the API says
which when that happens.`),
		PreRunE: cc.validateFlags,
		RunE:    cc.run,
		Example: `  # Set the topics destinations can subscribe to
  hookdeck outpost config set TOPICS=user.created,user.updated

  # Preview a change without applying it
  hookdeck outpost config set MAX_RETRY_LIMIT=5 --dry-run

  # Return a key to its default
  hookdeck outpost config set --unset MAX_RETRY_LIMIT`,
		Annotations: map[string]string{
			"cli.arguments": `[
				{"name":"KEY=VALUE","type":"string","description":"Configuration values to set. Repeatable.","required":false}
			]`,
		},
	}

	cc.cmd.Flags().StringArrayVar(&cc.unset, "unset", nil, "Return a key to its default (repeatable)")
	cc.cmd.Flags().StringVar(&cc.configFile, "config-file", "", "Path to a JSON file of configuration values")
	cc.cmd.Flags().BoolVar(&cc.dryRun, "dry-run", false, "Show what would change without applying it")
	cc.cmd.Flags().StringVar(&cc.output, "output", "", "Output format (json)")

	return cc
}

func (cc *outpostConfigSetCmd) validateFlags(cmd *cobra.Command, args []string) error {
	if err := rejectEmptyFlags(cmd); err != nil {
		return err
	}
	if len(args) == 0 && len(cc.unset) == 0 && cc.configFile == "" {
		return fmt.Errorf("nothing to change. Pass KEY=VALUE arguments, --unset, or --config-file")
	}
	if len(args) > 0 && cc.configFile != "" {
		return fmt.Errorf("KEY=VALUE arguments and --config-file cannot be used together")
	}
	return nil
}

func (cc *outpostConfigSetCmd) run(cmd *cobra.Command, args []string) error {
	update, err := cc.buildUpdate(args)
	if err != nil {
		return err
	}

	client := Config.GetOutpostAPIClient()
	ctx := context.Background()

	current, err := client.GetOutpostConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to read current config: %w", err)
	}

	if cc.dryRun {
		printOutpostConfigDiff(current, update)
		return nil
	}

	updated, err := client.UpdateOutpostConfig(ctx, update)
	if err != nil {
		return fmt.Errorf("failed to update config: %w", err)
	}

	if cc.output == "json" {
		return printJSONIndented(updated)
	}

	fmt.Printf("%s Updated %d configuration value(s)\n", SuccessCheck, len(update))
	fmt.Println("\nChanges take a short while to reach the deployment. Check with 'hookdeck outpost status'.")

	return nil
}

func (cc *outpostConfigSetCmd) buildUpdate(args []string) (hookdeck.OutpostManagedConfig, error) {
	update := hookdeck.OutpostManagedConfig{}

	if cc.configFile != "" {
		contents, err := os.ReadFile(cc.configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read --config-file: %w", err)
		}
		if err := json.Unmarshal(contents, &update); err != nil {
			return nil, fmt.Errorf("--config-file must contain a JSON object: %w", err)
		}
	}

	for _, arg := range args {
		key, value, found := strings.Cut(arg, "=")
		key = strings.TrimSpace(key)
		if !found || key == "" {
			return nil, fmt.Errorf("%q must be in KEY=VALUE form", arg)
		}
		v := value
		update[key] = &v
	}

	// A nil value is how the API is told to clear a key.
	for _, key := range cc.unset {
		update[strings.TrimSpace(key)] = nil
	}

	return update, nil
}

// printOutpostConfigDiff shows before and after for each key being changed.
func printOutpostConfigDiff(current, update hookdeck.OutpostManagedConfig) {
	color := ansi.Color(os.Stdout)

	keys := make([]string, 0, len(update))
	for key := range update {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	fmt.Printf("\nDry run — %d value(s) would change:\n\n", len(keys))
	for _, key := range keys {
		before := "(not set)"
		if v, ok := current[key]; ok && v != nil && *v != "" {
			before = *v
		}

		after := "(default)"
		if v := update[key]; v != nil {
			after = *v
		}

		if before == after {
			fmt.Printf("  %s: unchanged (%s)\n", key, before)
			continue
		}
		fmt.Printf("  %s:\n", color.Green(key))
		fmt.Printf("    before: %s\n", before)
		fmt.Printf("    after:  %s\n", after)
	}
	fmt.Println("\nRe-run without --dry-run to apply.")
}

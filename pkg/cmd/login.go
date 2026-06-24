package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/login"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type loginCmd struct {
	cmd         *cobra.Command
	interactive bool
	local       bool
	cli_key     string
}

func newLoginCmd() *loginCmd {
	lc := &loginCmd{}

	lc.cmd = &cobra.Command{
		Use:   "login",
		Args:  validators.NoArgs,
		Short: "Login to your Hookdeck account",
		Long: `Login to your Hookdeck account to setup the CLI.

With a guest Console profile (after hookdeck listen), hookdeck login opens the browser to sign you
up and keep your sandbox data.

Use --cli-key with a claimed CLI client key from the Hookdeck product (for example Event Gateway
onboarding or Console CLI authorization). The CLI validates the key and saves your config, replacing
a guest profile when present. Device login (hookdeck login without --cli-key) is required for guest
upgrade and sandbox retention.`,
		Example: `  $ hookdeck login
  $ hookdeck login --cli-key <key>
  $ hookdeck logout && hookdeck login  # existing Platform account
  $ hookdeck login -i  # interactive mode (no browser)
  $ hookdeck login --local  # save credentials to .hookdeck/config.toml`,
		RunE: lc.runLoginCmd,
	}
	lc.cmd.Flags().BoolVarP(&lc.interactive, "interactive", "i", false, "Run interactive configuration mode if you cannot open a browser")
	lc.cmd.Flags().BoolVar(&lc.local, "local", false, "Save credentials to current directory (.hookdeck/config.toml)")
	lc.cmd.Flags().StringVar(&lc.cli_key, "cli-key", "", "CLI key from Hookdeck dashboard onboarding")

	return lc
}

func (lc *loginCmd) runLoginCmd(cmd *cobra.Command, args []string) error {
	if lc.local && Config.ConfigFileFlag != "" {
		return fmt.Errorf("Error: --local and --hookdeck-config flags cannot be used together\n  --local creates config at: .hookdeck/config.toml\n  --hookdeck-config uses custom path: %s", Config.ConfigFileFlag)
	}

	cli_key := lc.cli_key
	if cli_key == "" {
		if cli_key_flag := cmd.Root().PersistentFlags().Lookup("cli-key"); cli_key_flag != nil && cli_key_flag.Changed {
			cli_key = Config.Profile.APIKey
		}
	}

	if cli_key != "" && lc.interactive {
		return fmt.Errorf("--cli-key cannot be used with --interactive")
	}

	var err error
	if cli_key != "" {
		err = login.ConfigureFromClaimedCliKey(&Config, cli_key)
	} else if lc.interactive {
		err = login.InteractiveLogin(&Config)
	} else {
		err = login.Login(&Config, os.Stdin)
	}
	if err != nil {
		return err
	}

	if lc.local {
		return saveLocalConfig()
	}

	return nil
}

// saveLocalConfig writes the current profile credentials to .hookdeck/config.toml
// and prints a security warning if the file is newly created.
func saveLocalConfig() error {
	isNewConfig, err := Config.UseProjectLocal(Config.Profile.ProjectId, Config.Profile.ProjectMode)
	if err != nil {
		return err
	}

	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}
	configPath := filepath.Join(workingDir, ".hookdeck/config.toml")

	color := ansi.Color(os.Stdout)
	if isNewConfig {
		fmt.Printf("Created: %s\n", configPath)
		fmt.Printf("\n%s\n", color.Yellow("Security:"))
		fmt.Printf("  Local config files contain credentials and should NOT be committed to source control.\n")
		fmt.Printf("  Add .hookdeck/ to your .gitignore file.\n")
	} else {
		fmt.Printf("Updated: %s\n", configPath)
	}

	return nil
}

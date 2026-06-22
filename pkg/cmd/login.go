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
	cmd            *cobra.Command
	interactive    bool
	local          bool
	claim_guest    bool
	login_existing bool
	create_account bool
}

func newLoginCmd() *loginCmd {
	lc := &loginCmd{}

	lc.cmd = &cobra.Command{
		Use:   "login",
		Args:  validators.NoArgs,
		Short: "Login to your Hookdeck account",
		Long:  `Login to your Hookdeck account to setup the CLI`,
		Example: `  $ hookdeck login
  $ hookdeck login -i  # interactive mode (no browser)
  $ hookdeck login --local  # save credentials to .hookdeck/config.toml`,
		RunE: lc.runLoginCmd,
	}
	lc.cmd.Flags().BoolVarP(&lc.interactive, "interactive", "i", false, "Run interactive configuration mode if you cannot open a browser")
	lc.cmd.Flags().BoolVar(&lc.local, "local", false, "Save credentials to current directory (.hookdeck/config.toml)")
	lc.cmd.Flags().BoolVar(&lc.claim_guest, "claim-guest", false, "Claim the current guest Console sandbox when signing up (default for guest profiles)")
	lc.cmd.Flags().BoolVar(&lc.login_existing, "login", false, "Log in to an existing Hookdeck account and attach the CLI")
	lc.cmd.Flags().BoolVar(&lc.create_account, "create-account", false, "Create a new Hookdeck account and abandon the guest sandbox")

	return lc
}

func (lc *loginCmd) runLoginCmd(cmd *cobra.Command, args []string) error {
	if lc.local && Config.ConfigFileFlag != "" {
		return fmt.Errorf("Error: --local and --hookdeck-config flags cannot be used together\n  --local creates config at: .hookdeck/config.toml\n  --hookdeck-config uses custom path: %s", Config.ConfigFileFlag)
	}

	var err error
	login_opts := login.Options{
		ClaimGuest:    lc.claim_guest,
		LoginExisting: lc.login_existing,
		CreateAccount: lc.create_account,
	}
	if lc.interactive {
		err = login.InteractiveLogin(&Config)
	} else {
		err = login.Login(&Config, os.Stdin, login_opts)
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

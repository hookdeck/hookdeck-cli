package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/login"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type ciCmd struct {
	cmd    *cobra.Command
	apiKey string
}

func newCICmd() *ciCmd {
	lc := &ciCmd{}

	lc.cmd = &cobra.Command{
		Use:   "ci",
		Args:  validators.NoArgs,
		Short: "Login to your Hookdeck account in CI",
		Long:  `Login to your Hookdeck account to forward events in CI`,
		RunE:  lc.runCICmd,
	}
	lc.cmd.Flags().StringVar(&lc.apiKey, "api-key", "", "Your API key to use for the command")

	return lc
}

func (lc *ciCmd) runCICmd(cmd *cobra.Command, args []string) error {
	return login.CILogin(&Config, lc.apiKey)
}

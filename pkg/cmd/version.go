package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
	"github.com/hookdeck/hookdeck-cli/pkg/version"
)

var versionCmd = &cobra.Command{
	Use:     "version",
	Args:    validators.NoArgs,
	Short:   "Get the version of the Hookdeck CLI",
	Long:    "Print the CLI version and check whether a new version is available.",
	Example: "  $ hookdeck version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(version.Template)

		version.CheckLatestVersion()
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

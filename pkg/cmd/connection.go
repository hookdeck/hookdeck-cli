package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

var connectionCmd = &cobra.Command{
	Use:   "connection",
	Args:  validators.NoArgs,
	Short: "Manage your connections",
}

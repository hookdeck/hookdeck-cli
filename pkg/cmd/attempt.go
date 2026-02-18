package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type attemptCmd struct {
	cmd *cobra.Command
}

func newAttemptCmd() *attemptCmd {
	ac := &attemptCmd{}

	ac.cmd = &cobra.Command{
		Use:     "attempt",
		Aliases: []string{"attempts"},
		Args:    validators.NoArgs,
		Short:   "Inspect delivery attempts",
		Long: `List or get attempts (single delivery tries for an event). Use --event-id to list attempts for an event.`,
	}

	ac.cmd.AddCommand(newAttemptListCmd().cmd)
	ac.cmd.AddCommand(newAttemptGetCmd().cmd)

	return ac
}

func addAttemptCmdTo(parent *cobra.Command) {
	parent.AddCommand(newAttemptCmd().cmd)
}

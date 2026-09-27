package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type transformationCmd struct {
	cmd *cobra.Command
}

func newTransformationCmd() *transformationCmd {
	tc := &transformationCmd{}

	tc.cmd = &cobra.Command{
		Use:     "transformation",
		Aliases: []string{"transformations"},
		Args:    validators.NoArgs,
		Short:   "Manage your transformations",
		Long: `Manage JavaScript transformations for request/response processing.

Transformations run custom code to modify event payloads. Create with --name and --code (or --code-file),
then attach to connections via rules. Use 'transformation run' to test code locally.`,
	}

	tc.cmd.AddCommand(newTransformationListCmd().cmd)
	tc.cmd.AddCommand(newTransformationGetCmd().cmd)
	tc.cmd.AddCommand(newTransformationCreateCmd().cmd)
	tc.cmd.AddCommand(newTransformationUpsertCmd().cmd)
	tc.cmd.AddCommand(newTransformationUpdateCmd().cmd)
	tc.cmd.AddCommand(newTransformationDeleteCmd().cmd)
	tc.cmd.AddCommand(newTransformationCountCmd().cmd)
	tc.cmd.AddCommand(newTransformationRunCmd().cmd)
	tc.cmd.AddCommand(newTransformationExecutionsCmd())

	return tc
}

// addTransformationCmdTo registers the transformation command tree on the given parent (e.g. gateway).
func addTransformationCmdTo(parent *cobra.Command) {
	parent.AddCommand(newTransformationCmd().cmd)
}

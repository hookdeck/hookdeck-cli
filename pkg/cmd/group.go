package cmd

import (
	"github.com/spf13/cobra"
)

// groupCommandAnnotation marks a command that exists only to group its
// subcommands: `hookdeck outpost`, `hookdeck outpost tenant`, and so on.
const groupCommandAnnotation = "hookdeck.com/group-command"

// markGroupCommands makes every group command in the tree report an unknown
// subcommand instead of printing help and exiting 0.
//
// THE BUG: `hookdeck project create` printed the `project` help and exited 0.
// v3.0.0 removed `project create/update/delete`, `hookdeck org` and the
// custom-domain commands, so every script still calling one carried on as
// though it had worked — a silent failure exactly where a loud one is needed.
//
// WHY Args ALONE DOES NOT FIX IT: cobra's Command.execute returns flag.ErrHelp
// for any command with no Run or RunE *before* it calls ValidateArgs, and
// ExecuteC turns flag.ErrHelp into "print help, return nil". So a validator on
// a non-runnable parent never runs — which is why `Args: validators.NoArgs` was
// already set on `project`, `gateway` and `outpost` and changed nothing. Only
// the root command errored, and only because Find falls back to legacyArgs for
// a command with no Args set, which reports unknown subcommands at the root.
//
// THE FIX: give the group a RunE that prints its help. That makes it runnable,
// so ValidateArgs runs and cobra.NoArgs reports
// `unknown command "create" for "hookdeck project"` with a non-zero exit, while
// a bare `hookdeck project` still prints help and exits 0.
//
// Applied by walking the assembled tree rather than at each of the two dozen
// constructors, so a group command added later cannot silently reintroduce the
// defect. TestUnknownSubcommandOfGroupCommandFails pins the behaviour.
func markGroupCommands(root *cobra.Command) {
	for _, child := range root.Commands() {
		// Runnable commands already validate their own arguments, and the root
		// command is handled by cobra's legacyArgs fallback.
		if child.HasSubCommands() && !child.Runnable() {
			markGroupCommand(child)
		}
		markGroupCommands(child)
	}
}

// markGroupCommand turns one command into a group command. See markGroupCommands.
func markGroupCommand(cmd *cobra.Command) {
	cmd.Args = cobra.NoArgs
	cmd.RunE = groupCommandHelp
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[groupCommandAnnotation] = "true"
}

// groupCommandHelp is what a group command does when invoked with no
// subcommand: the same help cobra printed before, and the same exit code.
func groupCommandHelp(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}

// isGroupCommand reports whether cmd only groups subcommands.
//
// The credential and project checks in gatewayPersistentPreRunE and
// outpostPersistentPreRunE consult this. Those hooks did not run for a group
// command while it was non-runnable, so `hookdeck outpost tenant` printed its
// help without an API key; making it runnable would otherwise have turned every
// group's help into an authentication error.
func isGroupCommand(cmd *cobra.Command) bool {
	return cmd != nil && cmd.Annotations[groupCommandAnnotation] == "true"
}

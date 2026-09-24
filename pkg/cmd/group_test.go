package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// groupCommandPaths lists every command in the real tree that exists only to
// group subcommands, as argv slices.
func groupCommandPaths(t *testing.T) [][]string {
	t.Helper()
	var out [][]string
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		for _, child := range c.Commands() {
			if child.HasSubCommands() {
				out = append(out, strings.Fields(child.CommandPath())[1:])
			}
			walk(child)
		}
	}
	walk(RootCmd())
	require.NotEmpty(t, out, "expected the command tree to contain group commands")
	return out
}

// runRoot executes the real root command with argv and returns its error and
// anything it printed. Output is captured so a passing run stays quiet.
func runRoot(t *testing.T, argv ...string) (error, string) {
	t.Helper()
	root := RootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(argv)
	t.Cleanup(func() {
		root.SetOut(nil)
		root.SetErr(nil)
		root.SetArgs(nil)
	})
	return root.Execute(), buf.String()
}

// TestUnknownSubcommandOfGroupCommandFails is the regression test for group
// commands that answered an unknown subcommand with help and exit 0.
//
// v3.0.0 removed `project create`, `hookdeck org` and the custom-domain
// commands. A script still calling one saw success and no error, which is the
// worst possible answer to "this command no longer exists".
//
// Every group command is covered, not just the three the bug was reported
// against: the cause is cobra's behaviour for any parent with no Run or RunE,
// so any group command left unmarked has the same defect.
func TestUnknownSubcommandOfGroupCommandFails(t *testing.T) {
	for _, path := range groupCommandPaths(t) {
		name := strings.Join(path, " ")
		t.Run(name, func(t *testing.T) {
			argv := append(append([]string{}, path...), "definitely-not-a-subcommand")
			err, _ := runRoot(t, argv...)
			require.Error(t, err, "`hookdeck %s definitely-not-a-subcommand` must fail, not print help and exit 0", name)
			assert.Contains(t, err.Error(), "unknown command",
				"the error has to say the subcommand is unknown; Execute keys its message off that")
		})
	}
}

// TestBareGroupCommandPrintsHelp pins the other half of the contract: only an
// *unknown* subcommand fails. `hookdeck outpost` still prints help and exits 0,
// and does so without credentials — the group commands acquired a RunE, which
// means their group's PersistentPreRunE now runs where it previously did not.
func TestBareGroupCommandPrintsHelp(t *testing.T) {
	for _, path := range groupCommandPaths(t) {
		name := strings.Join(path, " ")
		t.Run(name, func(t *testing.T) {
			err, out := runRoot(t, path...)
			require.NoError(t, err, "`hookdeck %s` must still print help and exit 0", name)
			assert.Contains(t, out, "Available Commands:",
				"`hookdeck %s` must print its help", name)
		})
	}
}

// TestRootRejectsUnknownCommand keeps the root-level behaviour that was already
// correct, so the group fix cannot regress it.
func TestRootRejectsUnknownCommand(t *testing.T) {
	err, _ := runRoot(t, "org")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command")
}

// TestUnknownCommandMessageNamesTheRightCommand covers the recovery text.
//
// It used to be built from os.Args[1] and the root command path, which is the
// unknown word only at the root. One level down, os.Args[1] is the *group*, so
// `hookdeck project create` reported `Unknown command "project" for "hookdeck".
// Did you mean "project"?` — every part of it wrong.
func TestUnknownCommandMessageNamesTheRightCommand(t *testing.T) {
	t.Run("nested", func(t *testing.T) {
		err, _ := runRoot(t, "project", "create")
		require.Error(t, err)
		msg := unknownCommandMessage(mustFind(t, "project"), err.Error())
		assert.Contains(t, msg, `Unknown command "create" for "hookdeck project".`)
		assert.Contains(t, msg, `See "hookdeck project --help"`)
		assert.NotContains(t, msg, "Did you mean")
	})

	t.Run("root", func(t *testing.T) {
		err, _ := runRoot(t, "org")
		require.Error(t, err)
		msg := unknownCommandMessage(RootCmd(), err.Error())
		assert.Contains(t, msg, `Unknown command "org" for "hookdeck".`)
		assert.Contains(t, msg, `See "hookdeck --help"`)
	})

	t.Run("suggests the closest subcommand", func(t *testing.T) {
		err, _ := runRoot(t, "outpost", "tenant", "lst")
		require.Error(t, err)
		msg := unknownCommandMessage(mustFind(t, "outpost", "tenant"), err.Error())
		assert.Contains(t, msg, `Did you mean "list"?`,
			"cobra returns candidates in registration order, so the message has to rank them")
	})
}

func TestClosestSuggestion(t *testing.T) {
	assert.Equal(t, "list", closestSuggestion("lst", []string{"get", "list", "delete"}))
	assert.Equal(t, "listen", closestSuggestion("list", []string{"logout", "listen"}),
		"a prefix match beats a shorter edit distance")
	assert.Equal(t, "", closestSuggestion("zzz", nil))
}

func mustFind(t *testing.T, path ...string) *cobra.Command {
	t.Helper()
	cmd, _, err := RootCmd().Find(path)
	require.NoError(t, err)
	return cmd
}

// TestGroupCommandsAreMarked checks the mechanism as well as the behaviour, so
// a group command that starts passing the tests above for some other reason
// still has to go through markGroupCommands.
func TestGroupCommandsAreMarked(t *testing.T) {
	for _, path := range groupCommandPaths(t) {
		name := strings.Join(path, " ")
		cmd, _, err := RootCmd().Find(path)
		require.NoError(t, err)
		assert.True(t, isGroupCommand(cmd), "`hookdeck %s` groups subcommands but is not marked", name)
	}
}

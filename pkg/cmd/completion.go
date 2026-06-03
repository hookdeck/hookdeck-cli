package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type completionCmd struct {
	cmd *cobra.Command

	shell string
}

func newCompletionCmd() *completionCmd {
	cc := &completionCmd{}

	cc.cmd = &cobra.Command{
		Use:   "completion",
		Short: "Generate bash and zsh completion scripts",
		Long: `Generate bash and zsh completion scripts.

The completion script is written to stdout. Source it directly in your current
shell session, or redirect it to a file loaded by your shell's startup config.

To load completions in your current session:

  source <(hookdeck completion --shell bash)
  source <(hookdeck completion --shell zsh)

To load completions for every session, write the script to the location your
shell loads completions from, for example:

  # bash (Linux)
  hookdeck completion --shell bash > /etc/bash_completion.d/hookdeck

  # zsh
  hookdeck completion --shell zsh > "${fpath[1]}/_hookdeck"

This command also runs on install when using Homebrew or Scoop.`,
		Args: validators.NoArgs,
		Example: `  $ hookdeck completion --shell zsh
  $ hookdeck completion --shell bash
  $ source <(hookdeck completion --shell bash)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return selectShell(cc.shell, cmd.OutOrStdout())
		},
	}

	cc.cmd.Flags().StringVar(&cc.shell, "shell", "", "The shell to generate completion commands for. Supports \"bash\" or \"zsh\"")

	return cc
}

func selectShell(shell string, out io.Writer) error {
	selected := shell
	if selected == "" {
		selected = detectShell()
	}

	switch {
	case selected == "zsh":
		return rootCmd.GenZshCompletion(out)
	case selected == "bash":
		return rootCmd.GenBashCompletion(out)
	default:
		return fmt.Errorf("could not automatically detect your shell. Please run the command with the `--shell` flag for either bash or zsh")
	}
}

func detectShell() string {
	shell := os.Getenv("SHELL")

	switch {
	case strings.Contains(shell, "zsh"):
		return "zsh"
	case strings.Contains(shell, "bash"):
		return "bash"
	default:
		return ""
	}
}

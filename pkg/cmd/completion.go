package cmd

import (
	"fmt"
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
		Long: `Generate a shell completion script for hookdeck.

The script is written to standard output. To enable completions in the
current shell session, source the output:

  $ source <(hookdeck completion --shell bash)
  $ source <(hookdeck completion --shell zsh)

To install completions permanently, redirect the output to your shell's
completion directory:

  bash:  hookdeck completion --shell bash > /usr/local/etc/bash_completion.d/hookdeck
  zsh:   hookdeck completion --shell zsh > "${fpath[1]}/_hookdeck"

When installed via Homebrew or Scoop, completions are installed automatically.`,
		Args: validators.NoArgs,
		Example: `  $ hookdeck completion --shell zsh
  $ hookdeck completion --shell bash`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCompletion(cmd, cc.shell)
		},
	}

	cc.cmd.Flags().StringVar(&cc.shell, "shell", "", "The shell to generate completion commands for. Supports \"bash\" or \"zsh\"")

	return cc
}

func runCompletion(cmd *cobra.Command, shell string) error {
	selected := shell
	if selected == "" {
		selected = detectShell()
	}

	out := cmd.OutOrStdout()

	switch selected {
	case "zsh":
		return rootCmd.GenZshCompletion(out)
	case "bash":
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

package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"runtime"

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
		Long:  "Generate bash and zsh completion scripts. This command runs on install when using Homebrew or Scoop. You can optionally run it when using binaries directly or without a package manager.",
		Args:  validators.NoArgs,
		Example: `  $ hookdeck completion --shell zsh
  $ hookdeck completion --shell bash`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return selectShell(cc.shell)
		},
	}

	cc.cmd.Flags().StringVar(&cc.shell, "shell", "", "The shell to generate completion commands for. Supports \"bash\" or \"zsh\"")

	return cc
}

const (
	instructionsHeader = `
Suggested next steps:
---------------------`

	zshCompletionInstructions = `
1. Move ` + "`hookdeck-completion.zsh`" + ` to the correct location:
    mkdir -p ~/.hookdeck
    mv hookdeck-completion.zsh ~/.hookdeck

2. Add the following lines to your ` + "`.zshrc`" + ` enabling shell completion for Hookdeck:
    fpath=(~/.hookdeck $fpath)
    autoload -Uz compinit && compinit -i

3. Source your ` + "`.zshrc`" + ` or open a new terminal session:
    source ~/.zshrc`

	bashCompletionInstructionsMac = `
Set up bash autocompletion on your system:
1. Install the bash autocompletion package:
     brew install bash-completion
2. Follow the post-install instructions displayed by Homebrew; add a line like the following to your bash profile:
     [[ -r "/usr/local/etc/profile.d/bash_completion.sh" ]] && . "/usr/local/etc/profile.d/bash_completion.sh"

Set up Hookdeck autocompletion:
3. Move ` + "`hookdeck-completion.bash`" + ` to the correct location:
    mkdir -p ~/.hookdeck
    mv hookdeck-completion.bash ~/.hookdeck

4. Add the following line to your bash profile, so that Hookdeck autocompletion will be enabled every time you start a new terminal session:
    source ~/.hookdeck/hookdeck-completion.bash

5. Either restart your terminal, or run the following command in your current session to enable immediately:
    source ~/.hookdeck/hookdeck-completion.bash`

	bashCompletionInstructionsLinux = `
1. Ensure bash autocompletion is installed on your system. Often, this means verifying that ` + "`/etc/profile.d/bash_completion.sh`" + ` exists, and is sourced by your bash profile; the location of this file varies across distributions of Linux.

2. Move ` + "`hookdeck-completion.bash`" + ` to the correct location:
    mkdir -p ~/.hookdeck
    mv hookdeck-completion.bash ~/.hookdeck

3. Add the following line to your bash profile, so that Hookdeck autocompletion will be enabled every time you start a new terminal session:
    source ~/.hookdeck/hookdeck-completion.bash

4. Either restart your terminal, or run the following command in your current session to enable immediately:
    source ~/.hookdeck/hookdeck-completion.bash`
)

func selectShell(shell string) error {
	selected := shell
	if selected == "" {
		selected = detectShell()
	}

	switch {
	case selected == "zsh":
		fmt.Println("Detected `zsh`, generating zsh completion file: hookdeck-completion.zsh")
		err := rootCmd.GenZshCompletionFile("hookdeck-completion.zsh")
		if err == nil {
			fmt.Printf("%s%s\n", instructionsHeader, zshCompletionInstructions)
		}
		return err
	case selected == "bash":
		fmt.Println("Detected `bash`, generating bash completion file: hookdeck-completion.bash")
		err := rootCmd.GenBashCompletionFile("hookdeck-completion.bash")
		if err == nil {
			if runtime.GOOS == "darwin" {
				fmt.Printf("%s%s\n", instructionsHeader, bashCompletionInstructionsMac)
			} else if runtime.GOOS == "linux" {
				fmt.Printf("%s%s\n", instructionsHeader, bashCompletionInstructionsLinux)
			}
		}
		return err
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

package login

import (
	"fmt"
	"os"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
)

// SuccessMessage returns the display message for a successfully authenticated user
func SuccessMessage(displayName string, teamName string, isConsole bool) string {
	color := ansi.Color(os.Stdout)

	if isConsole == true {
		return fmt.Sprintf(
			"Done! The Hookdeck CLI is configured with your console Sandbox",
		)
	}

	if displayName == "" {
		return fmt.Sprintf(
			"Done! The Hookdeck CLI is configured in project %s\n",
			color.Bold(teamName),
		)
	}

	return fmt.Sprintf(
		"Done! The Hookdeck CLI is configured for %s in project %s\n",
		color.Bold(displayName),
		color.Bold(teamName),
	)
}

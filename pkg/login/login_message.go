package login

import (
	"fmt"
	"os"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
)

// SuccessMessage returns the display message for a successfully authenticated user
func SuccessMessage(displayName string, email string, organizationName string, teamName string, isConsole bool) string {
	color := ansi.Color(os.Stdout)

	if isConsole == true {
		return fmt.Sprintf(
			"The Hookdeck CLI is configured with your console Sandbox",
		)
	}

	return fmt.Sprintf(
		"The Hookdeck CLI is configured for %s (%s) on project %s in organization %s\n",
		color.Bold(displayName),
		color.Bold(email),
		color.Bold(teamName),
		color.Bold(organizationName),
	)
}

package login

import (
	"fmt"
	"os"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
)

// SuccessMessage returns the display message for a successfully authenticated user
func SuccessMessage(displayName string, teamName string) string {
	color := ansi.Color(os.Stdout)
	return fmt.Sprintf(
		"Done! The Hookdeck CLI is configured for %s in workspace %s\n",
		color.Bold(displayName),
		color.Bold(teamName),
	)
}

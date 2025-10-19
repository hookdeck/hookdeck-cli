package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Color definitions matching current implementation
	colorGreen  = lipgloss.Color("2")   // Green for success
	colorRed    = lipgloss.Color("1")   // Red for errors
	colorYellow = lipgloss.Color("3")   // Yellow for warnings
	colorFaint  = lipgloss.Color("240") // Faint gray
	colorPurple = lipgloss.Color("5")   // Purple for brand accent
	colorCyan   = lipgloss.Color("6")   // Cyan for brand accent

	// Base styles
	faintStyle = lipgloss.NewStyle().
			Foreground(colorFaint)

	boldStyle = lipgloss.NewStyle().
			Bold(true)

	greenStyle = lipgloss.NewStyle().
			Foreground(colorGreen)

	redStyle = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true)

	yellowStyle = lipgloss.NewStyle().
			Foreground(colorYellow)

	cyanStyle = lipgloss.NewStyle().
			Foreground(colorCyan)

	// Brand styles
	brandStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("4")). // Blue
			Bold(true)

	brandAccentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("4")) // Blue

	// Component styles
	selectionIndicatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("7")) // White/default

	sectionTitleStyle = faintStyle.Copy()

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("7"))

	waitingDotStyle = greenStyle.Copy()

	connectingDotStyle = yellowStyle.Copy()

	dividerStyle = lipgloss.NewStyle().
			Foreground(colorFaint)

	// Status code color styles
	successStatusStyle = lipgloss.NewStyle().
				Foreground(colorGreen)

	errorStatusStyle = lipgloss.NewStyle().
				Foreground(colorRed)

	warningStatusStyle = lipgloss.NewStyle().
				Foreground(colorYellow)
)

// ColorizeStatus returns a styled status code string
func ColorizeStatus(status int) string {
	statusStr := fmt.Sprintf("%d", status)

	switch {
	case status >= 200 && status < 300:
		return successStatusStyle.Render(statusStr)
	case status >= 400:
		return errorStatusStyle.Render(statusStr)
	case status >= 300:
		return warningStatusStyle.Render(statusStr)
	default:
		return statusStr
	}
}

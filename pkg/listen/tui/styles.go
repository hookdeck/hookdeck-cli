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
	colorBlue   = lipgloss.Color("4")   // Blue for the brand header
	colorWhite  = lipgloss.Color("7")   // White/default for selection and status bar
)

// Base styles
var (
	faintStyle lipgloss.Style
	boldStyle  lipgloss.Style
	greenStyle lipgloss.Style
	redStyle   lipgloss.Style

	yellowStyle lipgloss.Style
	cyanStyle   lipgloss.Style

	// Brand styles
	brandStyle       lipgloss.Style
	brandAccentStyle lipgloss.Style

	// Component styles
	selectionIndicatorStyle lipgloss.Style
	sectionTitleStyle       lipgloss.Style
	statusBarStyle          lipgloss.Style
	waitingDotStyle         lipgloss.Style
	connectingDotStyle      lipgloss.Style
	dividerStyle            lipgloss.Style

	// Status code color styles
	successStatusStyle lipgloss.Style
	errorStatusStyle   lipgloss.Style
	warningStatusStyle lipgloss.Style
)

// colorEnabled records whether the TUI may emit ANSI decoration. The interactive
// renderer sets it from the same answer ansi.ShouldUseColors gives the compact
// renderer; see SetColorEnabled.
var colorEnabled = true

func init() {
	buildStyles()
}

// SetColorEnabled turns TUI decoration on or off, and must be called before the
// Bubble Tea program starts.
//
// #404: --color off reached only the compact renderer, because the TUI draws
// with lipgloss rather than pkg/ansi. A controlling-pty run with --color off
// still emitted 48 SGR sequences. With colour disabled every style below becomes
// a bare lipgloss.Style, which renders its input unchanged, so the frames carry
// no SGR bytes at all — bold and faint included, matching what --color off means
// everywhere else in the CLI.
func SetColorEnabled(enabled bool) {
	colorEnabled = enabled
	buildStyles()
}

// decorated returns the styled variant when colour is on and a plain style when
// it is off. Every style in this file is built through it so no decoration can
// be added that --color off fails to suppress.
func decorated(style lipgloss.Style) lipgloss.Style {
	if !colorEnabled {
		return lipgloss.NewStyle()
	}
	return style
}

func buildStyles() {
	faintStyle = decorated(lipgloss.NewStyle().Foreground(colorFaint))
	boldStyle = decorated(lipgloss.NewStyle().Bold(true))
	greenStyle = decorated(lipgloss.NewStyle().Foreground(colorGreen))
	redStyle = decorated(lipgloss.NewStyle().Foreground(colorRed).Bold(true))
	yellowStyle = decorated(lipgloss.NewStyle().Foreground(colorYellow))
	cyanStyle = decorated(lipgloss.NewStyle().Foreground(colorCyan))

	brandStyle = decorated(lipgloss.NewStyle().Foreground(colorBlue).Bold(true))
	brandAccentStyle = decorated(lipgloss.NewStyle().Foreground(colorBlue))

	selectionIndicatorStyle = decorated(lipgloss.NewStyle().Foreground(colorWhite))
	sectionTitleStyle = faintStyle
	statusBarStyle = decorated(lipgloss.NewStyle().Foreground(colorWhite))
	waitingDotStyle = greenStyle
	connectingDotStyle = yellowStyle
	dividerStyle = decorated(lipgloss.NewStyle().Foreground(colorFaint))

	successStatusStyle = decorated(lipgloss.NewStyle().Foreground(colorGreen))
	errorStatusStyle = decorated(lipgloss.NewStyle().Foreground(colorRed))
	warningStatusStyle = decorated(lipgloss.NewStyle().Foreground(colorYellow))
}

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

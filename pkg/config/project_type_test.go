package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTypeLabel(t *testing.T) {
	tests := []struct {
		projectType string
		expected    string
	}{
		{ProjectTypeEventGateway, ProjectLabelGateway},
		{ProjectTypeConsole, ProjectLabelConsole},
		{ProjectTypeOutpost, ProjectLabelOutpost},
		{"EVENT_GATEWAY", ProjectLabelGateway},
		{"unknown", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.projectType, func(t *testing.T) {
			assert.Equal(t, tt.expected, TypeLabel(tt.projectType))
		})
	}
}

func TestLabelToType(t *testing.T) {
	tests := []struct {
		label    string
		expected string
	}{
		{ProjectLabelGateway, ProjectTypeEventGateway},
		{ProjectLabelConsole, ProjectTypeConsole},
		{ProjectLabelOutpost, ProjectTypeOutpost},
		{"gateway", ProjectTypeEventGateway},
		{"Unknown", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			assert.Equal(t, tt.expected, LabelToType(tt.label))
		})
	}
}

func TestModeToType(t *testing.T) {
	tests := []struct {
		mode     string
		expected string
	}{
		{"inbound", ProjectTypeEventGateway},
		{OutboundMode, ProjectTypeEventGateway},
		{"console", ProjectTypeConsole},
		{"outpost", ProjectTypeOutpost},
		{"Inbound", ProjectTypeEventGateway},
		{"unknown", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			assert.Equal(t, tt.expected, ModeToType(tt.mode))
		})
	}
}

func TestTypeToLegacyMode(t *testing.T) {
	tests := []struct {
		projectType string
		expected    string
	}{
		// event_gateway covers both inbound and outbound; "inbound" is the
		// representative value written back to config.
		{ProjectTypeEventGateway, "inbound"},
		{ProjectTypeConsole, "console"},
		{ProjectTypeOutpost, "outpost"},
		{"OUTPOST", "outpost"},
		{"unknown", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.projectType, func(t *testing.T) {
			assert.Equal(t, tt.expected, TypeToLegacyMode(tt.projectType))
		})
	}
}

// TestNormalizeProjectType is the important one: it is the single door every
// value from disk or from a caller goes through, and it has to accept all three
// vocabularies the CLI has used - the API type, the display label written to
// project_type by older CLIs, and the legacy mode.
func TestNormalizeProjectType(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected string
	}{
		{"api type", ProjectTypeEventGateway, ProjectTypeEventGateway},
		{"api type outpost", ProjectTypeOutpost, ProjectTypeOutpost},
		{"display label from an older config", ProjectLabelGateway, ProjectTypeEventGateway},
		{"display label console", ProjectLabelConsole, ProjectTypeConsole},
		{"legacy mode inbound", "inbound", ProjectTypeEventGateway},
		{"legacy mode outbound", OutboundMode, ProjectTypeEventGateway},
		{"legacy mode outpost", "outpost", ProjectTypeOutpost},
		{"mixed case", "Event_Gateway", ProjectTypeEventGateway},
		{"surrounding space", "  outpost  ", ProjectTypeOutpost},
		{"unknown", "something_else", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, NormalizeProjectType(tt.value))
		})
	}
}

// TestTypeRoundTrip pins the deliberate lossiness: the API folds inbound and
// outbound into one type, so a round trip normalizes outbound to inbound. The
// type survives; the mode does not.
func TestTypeRoundTrip(t *testing.T) {
	for _, projectType := range []string{ProjectTypeEventGateway, ProjectTypeConsole, ProjectTypeOutpost} {
		t.Run(projectType, func(t *testing.T) {
			assert.Equal(t, projectType, LabelToType(TypeLabel(projectType)))
		})
	}

	assert.Equal(t, "inbound", TypeToLegacyMode(ModeToType(OutboundMode)),
		"outbound is expected to normalize to inbound through the type mapping")
	assert.Equal(t, "inbound", TypeToLegacyMode(ModeToType("inbound")))
}

func TestIsGatewayProject(t *testing.T) {
	// Console projects are Event Gateway projects with a different entry point.
	trueCases := []string{
		ProjectTypeEventGateway, ProjectTypeConsole,
		ProjectLabelGateway, ProjectLabelConsole,
		"inbound", "outbound", "EVENT_GATEWAY",
	}
	for _, v := range trueCases {
		t.Run("true_"+v, func(t *testing.T) {
			assert.True(t, IsGatewayProject(v))
		})
	}

	falseCases := []string{ProjectTypeOutpost, ProjectLabelOutpost, "", "unknown"}
	for _, v := range falseCases {
		t.Run("false_"+v, func(t *testing.T) {
			assert.False(t, IsGatewayProject(v))
		})
	}
}

func TestIsConsoleProject(t *testing.T) {
	assert.True(t, IsConsoleProject(ProjectTypeConsole, "", ""))
	assert.True(t, IsConsoleProject("", ProjectTypeConsole, ""), "falls through to the legacy product field")
	assert.True(t, IsConsoleProject("", "", "console"), "falls through to the legacy mode field")
	assert.False(t, IsConsoleProject(ProjectTypeEventGateway, ProjectTypeConsole, ""),
		"the first recognized value wins, so a newer field is not overridden by an older one")
	assert.False(t, IsConsoleProject("", "", ""))
}

// TestProjectTypeToJSON guards the user-facing values. `gateway` is what the CLI
// has always emitted in --output json and accepted in --type; the API type is
// deliberately not used here.
func TestProjectTypeToJSON(t *testing.T) {
	tests := []struct {
		value    string
		expected string
	}{
		{ProjectTypeEventGateway, "gateway"},
		{ProjectTypeOutpost, "outpost"},
		{ProjectTypeConsole, "console"},
		{ProjectLabelGateway, "gateway"},
		{"inbound", "gateway"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			assert.Equal(t, tt.expected, ProjectTypeToJSON(tt.value))
		})
	}
}

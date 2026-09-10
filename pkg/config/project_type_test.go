package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestModeToProjectType(t *testing.T) {
	tests := []struct {
		mode     string
		expected string
	}{
		{"inbound", ProjectTypeGateway},
		{"INBOUND", ProjectTypeGateway},
		{"console", ProjectTypeConsole},
		{"Console", ProjectTypeConsole},
		{"outpost", ProjectTypeOutpost},
		{"outbound", ProjectTypeGateway}, // same as inbound
		{"Outbound", ProjectTypeGateway},
		{"unknown", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			got := ModeToProjectType(tt.mode)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestProjectTypeToMode(t *testing.T) {
	tests := []struct {
		projectType string
		expected    string
	}{
		{ProjectTypeGateway, "inbound"},
		{ProjectTypeConsole, "console"},
		{ProjectTypeOutpost, "outpost"},
		{"", ""},
		{"Unknown", ""},
	}
	for _, tt := range tests {
		t.Run(tt.projectType, func(t *testing.T) {
			got := ProjectTypeToMode(tt.projectType)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestProductToProjectType(t *testing.T) {
	tests := []struct {
		product  string
		expected string
	}{
		{ProjectProductEventGateway, ProjectTypeGateway},
		{ProjectProductConsole, ProjectTypeConsole},
		{ProjectProductOutpost, ProjectTypeOutpost},
		{"EVENT_GATEWAY", ProjectTypeGateway},
		{"unknown", ""},
		// An empty product is the response-missing-the-field case. It must not
		// resolve to a type, and callers have to treat "" as "unknown", not as
		// a project that happens to be a Gateway.
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.product, func(t *testing.T) {
			assert.Equal(t, tt.expected, ProductToProjectType(tt.product))
		})
	}
}

func TestProjectTypeToProduct(t *testing.T) {
	tests := []struct {
		projectType string
		expected    string
	}{
		{ProjectTypeGateway, ProjectProductEventGateway},
		{ProjectTypeConsole, ProjectProductConsole},
		{ProjectTypeOutpost, ProjectProductOutpost},
		{"gateway", ProjectProductEventGateway},
		{"Unknown", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.projectType, func(t *testing.T) {
			assert.Equal(t, tt.expected, ProjectTypeToProduct(tt.projectType))
		})
	}
}

func TestProductToLegacyMode(t *testing.T) {
	tests := []struct {
		product  string
		expected string
	}{
		// event_gateway covers both inbound and outbound; "inbound" is the
		// representative value written back to config.
		{ProjectProductEventGateway, "inbound"},
		{ProjectProductConsole, "console"},
		{ProjectProductOutpost, "outpost"},
		{"OUTPOST", "outpost"},
		{"unknown", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.product, func(t *testing.T) {
			assert.Equal(t, tt.expected, ProductToLegacyMode(tt.product))
		})
	}
}

func TestModeToProduct(t *testing.T) {
	tests := []struct {
		mode     string
		expected string
	}{
		{"inbound", ProjectProductEventGateway},
		{OutboundMode, ProjectProductEventGateway},
		{"console", ProjectProductConsole},
		{"outpost", ProjectProductOutpost},
		{"Inbound", ProjectProductEventGateway},
		{"unknown", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			assert.Equal(t, tt.expected, ModeToProduct(tt.mode))
		})
	}
}

// TestProductRoundTrip pins the deliberate lossiness of the mapping: the public
// API folds inbound and outbound into one product, so a round trip through
// product normalizes outbound to inbound. Type survives; mode does not.
func TestProductRoundTrip(t *testing.T) {
	for _, projectType := range []string{ProjectTypeGateway, ProjectTypeConsole, ProjectTypeOutpost} {
		t.Run(projectType, func(t *testing.T) {
			assert.Equal(t, projectType, ProductToProjectType(ProjectTypeToProduct(projectType)))
		})
	}

	assert.Equal(t, "inbound", ProductToLegacyMode(ModeToProduct("outbound")),
		"outbound is expected to normalize to inbound through the product mapping")
	assert.Equal(t, "inbound", ProductToLegacyMode(ModeToProduct("inbound")))
}

func TestIsGatewayProject(t *testing.T) {
	// Gateway = inbound, outbound, or console (type or mode)
	trueCases := []string{ProjectTypeGateway, ProjectProductEventGateway, "inbound", "outbound", "console", ProjectTypeConsole}
	for _, v := range trueCases {
		t.Run("true_"+v, func(t *testing.T) {
			assert.True(t, IsGatewayProject(v))
		})
	}
	trueCases = append(trueCases, "EVENT_GATEWAY", "Inbound")
	falseCases := []string{ProjectTypeOutpost, ProjectProductOutpost, "", "unknown"}
	for _, v := range falseCases {
		t.Run("false_"+v, func(t *testing.T) {
			assert.False(t, IsGatewayProject(v))
		})
	}
}

func TestProjectTypeToJSON(t *testing.T) {
	tests := []struct {
		projectType string
		expected    string
	}{
		{ProjectTypeGateway, "gateway"},
		{ProjectTypeOutpost, "outpost"},
		{ProjectTypeConsole, "console"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.projectType, func(t *testing.T) {
			got := ProjectTypeToJSON(tt.projectType)
			assert.Equal(t, tt.expected, got)
		})
	}
}

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

func TestIsGatewayProject(t *testing.T) {
	// Gateway = inbound, outbound, or console (type or mode)
	trueCases := []string{ProjectTypeGateway, "inbound", "outbound", "console", ProjectTypeConsole}
	for _, v := range trueCases {
		t.Run("true_"+v, func(t *testing.T) {
			assert.True(t, IsGatewayProject(v))
		})
	}
	falseCases := []string{ProjectTypeOutpost, ""}
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

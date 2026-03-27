package mcp

import (
	"net/url"
	"testing"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/stretchr/testify/require"
)

func TestFormatCurrentProject(t *testing.T) {
	u, _ := url.Parse("https://api.example.com")
	tests := []struct {
		name     string
		client   *hookdeck.Client
		expected string
	}{
		{"empty", &hookdeck.Client{BaseURL: u}, "not set"},
		{"id only", &hookdeck.Client{BaseURL: u, ProjectID: "tm_abc"}, "tm_abc"},
		{"name only", &hookdeck.Client{BaseURL: u, ProjectName: "Demos / app"}, "Demos / app"},
		{"name and id", &hookdeck.Client{BaseURL: u, ProjectID: "tm_abc", ProjectName: "Demos / app"}, "Demos / app (tm_abc)"},
		{"org name and id", &hookdeck.Client{BaseURL: u, ProjectID: "tm_abc", ProjectOrg: "Demos", ProjectName: "app"}, "Demos / app (tm_abc)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, formatCurrentProject(tt.client))
		})
	}
}

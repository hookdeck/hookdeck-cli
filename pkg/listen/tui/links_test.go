package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// Construction rules are covered in pkg/listen/links; these tests only guard
// the wiring from Config fields to the shared helpers.

func linksTestConfig() *Config {
	return &Config{
		DashboardBaseURL: "https://dashboard.hookdeck.com",
		InspectBaseURL:   "https://inspect.hookdeck.com",
		ProjectID:        "tm_123",
		Sources:          []*hookdeck.Source{{ID: "src_abc123def456"}},
	}
}

func TestDashboardHomeURLUsesConfigFields(t *testing.T) {
	cfg := linksTestConfig()
	assert.Equal(t, "https://dashboard.hookdeck.com/events/cli?team_id=tm_123", dashboardHomeURL(cfg))

	cfg.ProjectType = "console"
	assert.Equal(t, "https://inspect.hookdeck.com/abc123def456", dashboardHomeURL(cfg))

	cfg.Sources = append(cfg.Sources, &hookdeck.Source{ID: "src_other000000"})
	assert.Equal(t, "https://inspect.hookdeck.com", dashboardHomeURL(cfg))
}

func TestEventDashboardURLUsesConfigFields(t *testing.T) {
	cfg := linksTestConfig()
	assert.Equal(t, "https://dashboard.hookdeck.com/events/evt_1?team_id=tm_123", eventDashboardURL(cfg, "evt_1"))

	cfg.ProjectType = "console"
	assert.Equal(t, "https://inspect.hookdeck.com/?event_id=evt_1&team_id=tm_123", eventDashboardURL(cfg, "evt_1"))
}

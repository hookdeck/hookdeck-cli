package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Construction rules are covered in pkg/listen/links; these tests only guard
// the wiring from Config fields to the shared helpers.

func TestDashboardHomeURLUsesConfigFields(t *testing.T) {
	cfg := &Config{
		DashboardBaseURL: "https://dashboard.hookdeck.com",
		ConsoleBaseURL:   "https://console.hookdeck.com",
		ProjectID:        "tm_123",
	}
	assert.Equal(t, "https://dashboard.hookdeck.com/events/cli?team_id=tm_123", dashboardHomeURL(cfg))

	cfg.ProjectMode = "console"
	assert.Equal(t, "https://console.hookdeck.com?team_id=tm_123", dashboardHomeURL(cfg))
}

func TestEventDashboardURLUsesConfigFields(t *testing.T) {
	cfg := &Config{
		DashboardBaseURL: "https://dashboard.hookdeck.com",
		ConsoleBaseURL:   "https://console.hookdeck.com",
		ProjectID:        "tm_123",
	}
	assert.Equal(t, "https://dashboard.hookdeck.com/events/evt_1?team_id=tm_123", eventDashboardURL(cfg, "evt_1"))

	cfg.ProjectMode = "console"
	assert.Equal(t, "https://console.hookdeck.com/?event_id=evt_1&team_id=tm_123", eventDashboardURL(cfg, "evt_1"))
}

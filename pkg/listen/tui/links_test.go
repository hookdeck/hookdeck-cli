package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDashboardHomeURL(t *testing.T) {
	base := &Config{
		DashboardBaseURL: "https://dashboard.hookdeck.com",
		ConsoleBaseURL:   "https://console.hookdeck.com",
	}

	t.Run("dashboard with project id", func(t *testing.T) {
		cfg := *base
		cfg.ProjectID = "tm_123"
		assert.Equal(t, "https://dashboard.hookdeck.com/events/cli?team_id=tm_123", dashboardHomeURL(&cfg))
	})

	t.Run("dashboard without project id omits team_id", func(t *testing.T) {
		cfg := *base
		assert.Equal(t, "https://dashboard.hookdeck.com/events/cli", dashboardHomeURL(&cfg))
	})

	t.Run("console with project id", func(t *testing.T) {
		cfg := *base
		cfg.ProjectMode = "console"
		cfg.ProjectID = "tm_123"
		assert.Equal(t, "https://console.hookdeck.com?team_id=tm_123", dashboardHomeURL(&cfg))
	})

	t.Run("console without project id omits team_id", func(t *testing.T) {
		cfg := *base
		cfg.ProjectMode = "console"
		assert.Equal(t, "https://console.hookdeck.com", dashboardHomeURL(&cfg))
	})
}

func TestEventDashboardURL(t *testing.T) {
	base := &Config{
		DashboardBaseURL: "https://dashboard.hookdeck.com",
		ConsoleBaseURL:   "https://console.hookdeck.com",
	}

	t.Run("dashboard with project id", func(t *testing.T) {
		cfg := *base
		cfg.ProjectID = "tm_123"
		assert.Equal(t, "https://dashboard.hookdeck.com/events/evt_1?team_id=tm_123", eventDashboardURL(&cfg, "evt_1"))
	})

	t.Run("dashboard without project id omits team_id", func(t *testing.T) {
		cfg := *base
		assert.Equal(t, "https://dashboard.hookdeck.com/events/evt_1", eventDashboardURL(&cfg, "evt_1"))
	})

	t.Run("console with project id", func(t *testing.T) {
		cfg := *base
		cfg.ProjectMode = "console"
		cfg.ProjectID = "tm_123"
		assert.Equal(t, "https://console.hookdeck.com/?event_id=evt_1&team_id=tm_123", eventDashboardURL(&cfg, "evt_1"))
	})

	t.Run("console without project id omits team_id", func(t *testing.T) {
		cfg := *base
		cfg.ProjectMode = "console"
		assert.Equal(t, "https://console.hookdeck.com/?event_id=evt_1", eventDashboardURL(&cfg, "evt_1"))
	})
}

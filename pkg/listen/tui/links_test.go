package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/websocket"
)

// Construction rules are covered in pkg/listen/links; these tests only guard
// the wiring from Config fields to the shared helpers.

func linksTestConfig() *Config {
	source := &hookdeck.Source{ID: "src_abc123def456"}
	return &Config{
		DashboardBaseURL: "https://dashboard.hookdeck.com",
		InspectBaseURL:   "https://inspect.hookdeck.com",
		ProjectID:        "tm_123",
		Sources:          []*hookdeck.Source{source},
		Connections:      []*hookdeck.Connection{{ID: "web_1", Source: source}},
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
	event := EventInfo{
		ID:   "evt_1",
		Data: &websocket.Attempt{Body: websocket.AttemptBody{EventID: "evt_1", ConnectionId: "web_1", RequestID: "req_1"}},
	}
	assert.Equal(t, "https://dashboard.hookdeck.com/events/evt_1?team_id=tm_123", eventDashboardURL(cfg, event))

	cfg.ProjectType = "console"
	assert.Equal(t, "https://inspect.hookdeck.com/abc123def456?request=req_1", eventDashboardURL(cfg, event))

	event.Data.Body.RequestID = ""
	assert.Equal(t, "https://inspect.hookdeck.com/abc123def456", eventDashboardURL(cfg, event))
}

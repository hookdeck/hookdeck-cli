package links

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/websocket"
)

const (
	dashboardBase = "https://dashboard.hookdeck.com"
	inspectBase   = "https://inspect.hookdeck.com"
)

func dashboardTarget(projectID string) Target {
	return Target{DashboardBaseURL: dashboardBase, InspectBaseURL: inspectBase, ProjectType: "gateway", ProjectID: projectID}
}

func inspectTarget(projectID string) Target {
	return Target{DashboardBaseURL: dashboardBase, InspectBaseURL: inspectBase, ProjectType: "console", ProjectID: projectID}
}

func TestHome(t *testing.T) {
	t.Run("dashboard with project id", func(t *testing.T) {
		assert.Equal(t, "https://dashboard.hookdeck.com/events/cli?team_id=tm_123", dashboardTarget("tm_123").Home("src_abc123def456"))
	})

	t.Run("dashboard without project id omits team_id", func(t *testing.T) {
		assert.Equal(t, "https://dashboard.hookdeck.com/events/cli", dashboardTarget("").Home(""))
	})

	t.Run("inspect with one source links to the source page without src_ or team_id", func(t *testing.T) {
		assert.Equal(t, "https://inspect.hookdeck.com/abc123def456", inspectTarget("tm_123").Home("src_abc123def456"))
	})

	t.Run("inspect without a single source links to inspect home", func(t *testing.T) {
		assert.Equal(t, "https://inspect.hookdeck.com", inspectTarget("tm_123").Home(""))
	})

	t.Run("inspect base with trailing slash", func(t *testing.T) {
		target := inspectTarget("")
		target.InspectBaseURL = "https://inspect-staging.hookdeck.com/"
		assert.Equal(t, "https://inspect-staging.hookdeck.com/abc123def456", target.Home("src_abc123def456"))
	})
}

func TestHomeDisplay(t *testing.T) {
	t.Run("dashboard shows events path without team_id", func(t *testing.T) {
		assert.Equal(t, "https://dashboard.hookdeck.com/events/cli", dashboardTarget("tm_123").HomeDisplay("src_abc123def456"))
	})

	t.Run("inspect shows the source page", func(t *testing.T) {
		assert.Equal(t, "https://inspect.hookdeck.com/abc123def456", inspectTarget("tm_123").HomeDisplay("src_abc123def456"))
	})

	t.Run("inspect without a single source shows inspect home", func(t *testing.T) {
		assert.Equal(t, "https://inspect.hookdeck.com", inspectTarget("tm_123").HomeDisplay(""))
	})
}

func TestEventURL(t *testing.T) {
	event := Event{ID: "evt_1", SourceID: "src_abc123def456", RequestID: "req_1"}

	t.Run("dashboard with project id", func(t *testing.T) {
		assert.Equal(t, "https://dashboard.hookdeck.com/events/evt_1?team_id=tm_123", dashboardTarget("tm_123").EventURL(event))
	})

	t.Run("dashboard without project id omits team_id", func(t *testing.T) {
		assert.Equal(t, "https://dashboard.hookdeck.com/events/evt_1", dashboardTarget("").EventURL(event))
	})

	t.Run("inspect with source and request selects the request", func(t *testing.T) {
		assert.Equal(t, "https://inspect.hookdeck.com/abc123def456?request=req_1", inspectTarget("tm_123").EventURL(event))
	})

	t.Run("inspect without request id links to the source page", func(t *testing.T) {
		assert.Equal(t, "https://inspect.hookdeck.com/abc123def456", inspectTarget("tm_123").EventURL(Event{ID: "evt_1", SourceID: "src_abc123def456"}))
	})

	t.Run("inspect with unknown source falls back to inspect home", func(t *testing.T) {
		assert.Equal(t, "https://inspect.hookdeck.com", inspectTarget("tm_123").EventURL(Event{ID: "evt_1", RequestID: "req_1"}))
	})
}

func TestEventDisplay(t *testing.T) {
	event := Event{ID: "evt_1", SourceID: "src_abc123def456", RequestID: "req_1"}

	t.Run("dashboard shows the event page without team_id", func(t *testing.T) {
		assert.Equal(t, "https://dashboard.hookdeck.com/events/evt_1", dashboardTarget("tm_123").EventDisplay(event))
	})

	t.Run("inspect shows the same link it opens", func(t *testing.T) {
		assert.Equal(t, "https://inspect.hookdeck.com/abc123def456?request=req_1", inspectTarget("tm_123").EventDisplay(event))
	})
}

func TestSessionSourceID(t *testing.T) {
	one := &hookdeck.Source{ID: "src_1"}
	two := &hookdeck.Source{ID: "src_2"}

	assert.Equal(t, "src_1", SessionSourceID([]*hookdeck.Source{one}))
	assert.Equal(t, "", SessionSourceID([]*hookdeck.Source{one, two}), "several sources have no single page")
	assert.Equal(t, "", SessionSourceID(nil))
}

func TestAttemptEvent(t *testing.T) {
	connections := []*hookdeck.Connection{
		{ID: "web_a", Source: &hookdeck.Source{ID: "src_a"}},
		{ID: "web_b", Source: &hookdeck.Source{ID: "src_b"}},
	}

	t.Run("resolves the source from webhook_id and carries request_id", func(t *testing.T) {
		attempt := &websocket.Attempt{Body: websocket.AttemptBody{EventID: "evt_1", ConnectionId: "web_b", RequestID: "req_1"}}
		assert.Equal(t, Event{ID: "evt_1", SourceID: "src_b", RequestID: "req_1"}, AttemptEvent("evt_1", attempt, connections))
	})

	t.Run("unknown connection leaves the source empty", func(t *testing.T) {
		attempt := &websocket.Attempt{Body: websocket.AttemptBody{EventID: "evt_1", ConnectionId: "web_zzz"}}
		assert.Equal(t, Event{ID: "evt_1"}, AttemptEvent("evt_1", attempt, connections))
	})

	t.Run("nil attempt", func(t *testing.T) {
		assert.Equal(t, Event{ID: "evt_1"}, AttemptEvent("evt_1", nil, connections))
	})
}

package links

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	dashboardBase = "https://dashboard.hookdeck.com"
	consoleBase   = "https://console.hookdeck.com"
)

func TestDashboardHome(t *testing.T) {
	t.Run("dashboard with project id", func(t *testing.T) {
		assert.Equal(t, "https://dashboard.hookdeck.com/events/cli?team_id=tm_123", DashboardHome(dashboardBase, consoleBase, "inbound", "tm_123"))
	})

	t.Run("dashboard without project id omits team_id", func(t *testing.T) {
		assert.Equal(t, "https://dashboard.hookdeck.com/events/cli", DashboardHome(dashboardBase, consoleBase, "inbound", ""))
	})

	t.Run("console with project id", func(t *testing.T) {
		assert.Equal(t, "https://console.hookdeck.com?team_id=tm_123", DashboardHome(dashboardBase, consoleBase, "console", "tm_123"))
	})

	t.Run("console without project id omits team_id", func(t *testing.T) {
		assert.Equal(t, "https://console.hookdeck.com", DashboardHome(dashboardBase, consoleBase, "console", ""))
	})
}

func TestDashboardHomeDisplay(t *testing.T) {
	t.Run("dashboard shows events path", func(t *testing.T) {
		assert.Equal(t, "https://dashboard.hookdeck.com/events/cli", DashboardHomeDisplay(dashboardBase, consoleBase, "inbound"))
	})

	t.Run("console shows console base", func(t *testing.T) {
		assert.Equal(t, "https://console.hookdeck.com", DashboardHomeDisplay(dashboardBase, consoleBase, "console"))
	})
}

func TestEvent(t *testing.T) {
	t.Run("dashboard with project id", func(t *testing.T) {
		assert.Equal(t, "https://dashboard.hookdeck.com/events/evt_1?team_id=tm_123", Event(dashboardBase, consoleBase, "inbound", "tm_123", "evt_1"))
	})

	t.Run("dashboard without project id omits team_id", func(t *testing.T) {
		assert.Equal(t, "https://dashboard.hookdeck.com/events/evt_1", Event(dashboardBase, consoleBase, "inbound", "", "evt_1"))
	})

	t.Run("console with project id", func(t *testing.T) {
		assert.Equal(t, "https://console.hookdeck.com/?event_id=evt_1&team_id=tm_123", Event(dashboardBase, consoleBase, "console", "tm_123", "evt_1"))
	})

	t.Run("console without project id omits team_id", func(t *testing.T) {
		assert.Equal(t, "https://console.hookdeck.com/?event_id=evt_1", Event(dashboardBase, consoleBase, "console", "", "evt_1"))
	})
}

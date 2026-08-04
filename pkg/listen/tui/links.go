package tui

import "github.com/hookdeck/hookdeck-cli/pkg/listen/links"

// dashboardHomeURL returns the dashboard (or console) events link for the
// session. See pkg/listen/links for the shared construction rules.
func dashboardHomeURL(cfg *Config) string {
	return links.DashboardHome(cfg.DashboardBaseURL, cfg.ConsoleBaseURL, cfg.ProjectMode, cfg.ProjectID)
}

// eventDashboardURL returns the dashboard (or console) deep-link for a single
// event. See pkg/listen/links for the shared construction rules.
func eventDashboardURL(cfg *Config, eventID string) string {
	return links.Event(cfg.DashboardBaseURL, cfg.ConsoleBaseURL, cfg.ProjectMode, cfg.ProjectID, eventID)
}

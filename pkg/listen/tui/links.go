package tui

import "github.com/hookdeck/hookdeck-cli/pkg/listen/links"

// linkTarget returns where this session's links point. See pkg/listen/links
// for the shared construction rules.
func linkTarget(cfg *Config) links.Target {
	return links.Target{
		DashboardBaseURL: cfg.DashboardBaseURL,
		InspectBaseURL:   cfg.InspectBaseURL,
		ProjectType:      cfg.ProjectType,
		ProjectID:        cfg.ProjectID,
	}
}

// homeURL returns the link to the session's events: the dashboard
// events view, or for guest accounts the Inspect page of the session's Source
// (Inspect home when it listens to several).
func homeURL(cfg *Config) string {
	return linkTarget(cfg).Home(links.SessionSourceID(cfg.Sources))
}

// eventURL returns the deep-link for a single event: the dashboard
// event page, or for guest accounts the Inspect link that opens the event.
func eventURL(cfg *Config, eventID string) string {
	return linkTarget(cfg).EventURL(eventID)
}

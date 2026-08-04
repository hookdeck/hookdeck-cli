package tui

// dashboardHomeURL returns the dashboard (or console) events link for the
// session, omitting the team_id parameter when the project id is unknown so
// links never render with an empty value.
func dashboardHomeURL(cfg *Config) string {
	if cfg.ProjectMode == "console" {
		if cfg.ProjectID == "" {
			return cfg.ConsoleBaseURL
		}
		return cfg.ConsoleBaseURL + "?team_id=" + cfg.ProjectID
	}
	if cfg.ProjectID == "" {
		return cfg.DashboardBaseURL + "/events/cli"
	}
	return cfg.DashboardBaseURL + "/events/cli?team_id=" + cfg.ProjectID
}

// eventDashboardURL returns the dashboard (or console) deep-link for a single
// event, omitting the team_id parameter when the project id is unknown.
func eventDashboardURL(cfg *Config, eventID string) string {
	if cfg.ProjectMode == "console" {
		url := cfg.ConsoleBaseURL + "/?event_id=" + eventID
		if cfg.ProjectID != "" {
			url += "&team_id=" + cfg.ProjectID
		}
		return url
	}
	url := cfg.DashboardBaseURL + "/events/" + eventID
	if cfg.ProjectID != "" {
		url += "?team_id=" + cfg.ProjectID
	}
	return url
}

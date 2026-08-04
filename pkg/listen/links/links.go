// Package links builds dashboard/console deep-links for listen output. It is
// shared by the interactive TUI and the compact/quiet printer so the two
// output modes produce identical, correctly scoped URLs.
package links

// DashboardHome returns the dashboard (or console) events link for the
// session, omitting the team_id parameter when the project id is unknown so
// links never render with an empty value.
func DashboardHome(dashboardBaseURL, consoleBaseURL, projectMode, projectID string) string {
	if projectMode == "console" {
		if projectID == "" {
			return consoleBaseURL
		}
		return consoleBaseURL + "?team_id=" + projectID
	}
	if projectID == "" {
		return dashboardBaseURL + "/events/cli"
	}
	return dashboardBaseURL + "/events/cli?team_id=" + projectID
}

// DashboardHomeDisplay returns the display text for the DashboardHome link:
// the same destination without the team_id query parameter.
func DashboardHomeDisplay(dashboardBaseURL, consoleBaseURL, projectMode string) string {
	if projectMode == "console" {
		return consoleBaseURL
	}
	return dashboardBaseURL + "/events/cli"
}

// Event returns the dashboard (or console) deep-link for a single event,
// omitting the team_id parameter when the project id is unknown.
func Event(dashboardBaseURL, consoleBaseURL, projectMode, projectID, eventID string) string {
	if projectMode == "console" {
		url := consoleBaseURL + "/?event_id=" + eventID
		if projectID != "" {
			url += "&team_id=" + projectID
		}
		return url
	}
	url := dashboardBaseURL + "/events/" + eventID
	if projectID != "" {
		url += "?team_id=" + projectID
	}
	return url
}

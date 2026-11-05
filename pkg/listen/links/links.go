// Package links builds the web links listen prints: Event Gateway dashboard
// links for regular projects, and Hookdeck Inspect links for guest accounts
// (project type "console"). It is shared by the interactive TUI and the
// compact/quiet output so every output mode produces identical, correctly
// scoped URLs.
package links

import (
	"net/url"
	"strings"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// Target describes where a listen session's links point.
type Target struct {
	// DashboardBaseURL is the Event Gateway dashboard, used for every project
	// type except console.
	DashboardBaseURL string
	// InspectBaseURL is Hookdeck Inspect, used for guest (console) projects.
	InspectBaseURL string
	// ProjectType is the profile's project type (config.ProjectType*).
	ProjectType string
	// ProjectID is the project's team id. Dashboard links are scoped by it,
	// and Inspect event links carry it so Inspect can check the visitor is in
	// that project.
	ProjectID string
}

func (t Target) inspect() bool {
	return t.ProjectType == config.ProjectTypeConsole
}

// Name returns what the session's links open, for the labels shown next to
// them: "Hookdeck Inspect" for guest accounts, "dashboard" otherwise.
func (t Target) Name() string {
	if t.inspect() {
		return "Hookdeck Inspect"
	}
	return "dashboard"
}

// HomeHint returns the sentence printed before the Home link. The dashboard
// wording is unchanged; Inspect shows the requests a Source received.
func (t Target) HomeHint(dashboardHint string) string {
	if t.inspect() {
		return "View your requests in Hookdeck Inspect"
	}
	return dashboardHint
}

// Home returns the link to the session's events. For the dashboard that is
// the CLI events view, scoped by team_id when the project id is known. For
// Inspect it is the page of sourceID, or the Inspect home page when sourceID
// is empty (a session listening to several Sources has no single page).
func (t Target) Home(sourceID string) string {
	if t.inspect() {
		return inspectSource(t.InspectBaseURL, sourceID)
	}
	if t.ProjectID == "" {
		return t.DashboardBaseURL + "/events/cli"
	}
	return t.DashboardBaseURL + "/events/cli?team_id=" + t.ProjectID
}

// HomeDisplay returns the display text for the Home link: the same
// destination without the team_id query parameter.
func (t Target) HomeDisplay(sourceID string) string {
	if t.inspect() {
		return inspectSource(t.InspectBaseURL, sourceID)
	}
	return t.DashboardBaseURL + "/events/cli"
}

// EventURL returns the deep-link for a single event. For the dashboard that is
// the event page, scoped by team_id when the project id is known. For Inspect
// it is the home page with ?event_id= and ?team_id=: Inspect looks the event
// up and opens the request that created it on its Source's page.
func (t Target) EventURL(eventID string) string {
	if t.inspect() {
		return inspectEvent(t.InspectBaseURL, eventID, t.ProjectID)
	}
	link := t.DashboardBaseURL + "/events/" + eventID
	if t.ProjectID != "" {
		link += "?team_id=" + t.ProjectID
	}
	return link
}

// EventDisplay returns the display text for the EventURL link, as printed at
// the end of each event log line. The dashboard shows the event page without
// team_id. Inspect shows the full link, team_id included, because Inspect
// needs both ids to open the event.
func (t Target) EventDisplay(eventID string) string {
	if t.inspect() {
		return t.EventURL(eventID)
	}
	return t.DashboardBaseURL + "/events/" + eventID
}

// inspectSource builds an Inspect Source page link. Inspect addresses a Source
// by its id without the "src_" prefix. Without a Source it falls back to the
// Inspect home page.
func inspectSource(inspectBaseURL, sourceID string) string {
	bare := strings.TrimPrefix(sourceID, "src_")
	if bare == "" {
		return inspectBaseURL
	}
	return strings.TrimSuffix(inspectBaseURL, "/") + "/" + url.PathEscape(bare)
}

// inspectEvent builds an Inspect event link: the home page with event_id and,
// when known, team_id. These are the parameters Console's event links used.
func inspectEvent(inspectBaseURL, eventID, teamID string) string {
	query := url.Values{}
	query.Set("event_id", eventID)
	if teamID != "" {
		query.Set("team_id", teamID)
	}
	return strings.TrimSuffix(inspectBaseURL, "/") + "/?" + query.Encode()
}

// SessionSourceID returns the id of the one Source a session listens to, or
// "" when it listens to none or to several.
func SessionSourceID(sources []*hookdeck.Source) string {
	if len(sources) != 1 || sources[0] == nil {
		return ""
	}
	return sources[0].ID
}

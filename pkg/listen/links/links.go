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
	"github.com/hookdeck/hookdeck-cli/pkg/websocket"
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
	// ProjectID scopes dashboard links with team_id. Inspect links ignore it:
	// an Inspect page is addressed by its Source alone.
	ProjectID string
}

// Event identifies one event a link points at. SourceID and RequestID only
// matter for Inspect, which has no event page: an event is shown as the
// request that created it, on its Source's page.
type Event struct {
	ID        string
	SourceID  string
	RequestID string
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
		return inspectSource(t.InspectBaseURL, sourceID, "")
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
		return inspectSource(t.InspectBaseURL, sourceID, "")
	}
	return t.DashboardBaseURL + "/events/cli"
}

// EventURL returns the deep-link for a single event. For the dashboard that is
// the event page, scoped by team_id when the project id is known. For Inspect
// it is the event's Source page, selecting its request when the request id is
// known, and the Inspect home page when the Source is unknown.
func (t Target) EventURL(event Event) string {
	if t.inspect() {
		return inspectSource(t.InspectBaseURL, event.SourceID, event.RequestID)
	}
	link := t.DashboardBaseURL + "/events/" + event.ID
	if t.ProjectID != "" {
		link += "?team_id=" + t.ProjectID
	}
	return link
}

// EventDisplay returns the display text for the EventURL link, as printed at
// the end of each event log line: the same destination without the team_id
// query parameter.
func (t Target) EventDisplay(event Event) string {
	if t.inspect() {
		return t.EventURL(event)
	}
	return t.DashboardBaseURL + "/events/" + event.ID
}

// inspectSource builds an Inspect Source page link. Inspect addresses a Source
// by its id without the "src_" prefix, and selects a request with ?request=.
// Without a Source it falls back to the Inspect home page.
func inspectSource(inspectBaseURL, sourceID, requestID string) string {
	bare := strings.TrimPrefix(sourceID, "src_")
	if bare == "" {
		return inspectBaseURL
	}
	link := strings.TrimSuffix(inspectBaseURL, "/") + "/" + url.PathEscape(bare)
	if requestID != "" {
		link += "?request=" + url.QueryEscape(requestID)
	}
	return link
}

// SessionSourceID returns the id of the one Source a session listens to, or
// "" when it listens to none or to several.
func SessionSourceID(sources []*hookdeck.Source) string {
	if len(sources) != 1 || sources[0] == nil {
		return ""
	}
	return sources[0].ID
}

// AttemptEvent returns the Event an attempt delivers: its event id, the
// Source of its connection (resolved from the connections the session
// loaded), and its request id when the server sent one.
func AttemptEvent(eventID string, attempt *websocket.Attempt, connections []*hookdeck.Connection) Event {
	event := Event{ID: eventID}
	if attempt == nil {
		return event
	}
	event.SourceID = ConnectionSourceID(connections, attempt.Body.ConnectionId)
	event.RequestID = attempt.Body.RequestID
	return event
}

// ConnectionSourceID returns the id of the Source that connectionID belongs
// to, from the connections the session loaded, or "" when it is not among
// them. An attempt names its connection (webhook_id) but not its Source.
func ConnectionSourceID(connections []*hookdeck.Connection, connectionID string) string {
	if connectionID == "" {
		return ""
	}
	for _, connection := range connections {
		if connection != nil && connection.ID == connectionID && connection.Source != nil {
			return connection.Source.ID
		}
	}
	return ""
}

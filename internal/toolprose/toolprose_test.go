package toolprose

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

// The guards built on this are only worth what the detector catches, and a
// detector that quietly stops matching is worse than none: the tests keep
// passing while the prose drifts. These are the real sentences from both
// servers, before and after the fix.
func TestUnknownActionMentions(t *testing.T) {
	vocabulary := []string{
		"list", "get", "upsert", "delete", "token", "portal", "create", "update",
		"enable", "disable", "retry", "run", "pause", "unpause", "cancel", "dismiss",
	}

	tests := []struct {
		name    string
		text    string
		offered []string
		want    []string
	}{
		{
			name:    "slash list after required for",
			text:    "Tenant ID. Required for get/upsert/delete/token/portal.",
			offered: []string{"list", "get"},
			want:    []string{"delete", "portal", "token", "upsert"},
		},
		{
			name:    "on <action> clause",
			text:    "Event ID. On list, filters by event ID(s).",
			offered: []string{"retry"},
			want:    []string{"list"},
		},
		{
			name:    "filters on list",
			text:    "Destination ID. Filters on list; links the destination on create/upsert/update.",
			offered: []string{"create", "upsert", "update"},
			want:    []string{"list"},
		},
		{
			name:    "comma list in parentheses",
			text:    "Bulk job ID (get, cancel).",
			offered: []string{"create", "cancel"},
			want:    []string{"get"},
		},
		{
			name:    "trailing action scope in parentheses",
			text:    "Max results (list)",
			offered: []string{"create"},
			want:    []string{"list"},
		},
		{
			name:    "<action> action only",
			text:    "Connections to re-route the request through (retry action only).",
			offered: []string{"get"},
			want:    []string{"retry"},
		},
		{
			name:    "everything it names is offered",
			text:    "Tenant ID. Required for upsert/delete/token/portal.",
			offered: []string{"upsert", "delete", "token", "portal"},
			want:    nil,
		},
		{
			name: "ordinary prose is not an action mention",
			// "get", "list", "set" and "run" are all ordinary English, which is
			// why this looks only at the forms the servers use to name an action.
			text:    "Get one from gateway_events_read list, or from a request's request_id. Omit to read everything that is set.",
			offered: []string{"upsert"},
			want:    nil,
		},
		{
			name:    "a word outside the resource's vocabulary is prose",
			text:    "Tenant IDs are chosen by the operator, so upsert is the way to create one.",
			offered: []string{"list", "get"},
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, UnknownActionMentions(tt.text, tt.offered, vocabulary))
		})
	}
}

func TestArrayPromises(t *testing.T) {
	problems := ArrayPromises(map[string]mcpcore.Prop{
		"topic":  {Type: "string", Desc: "Filter by topic(s). Accepts an array of strings or a comma-separated string."},
		"topics": {Type: "array", Desc: `Topics to subscribe to, or ["*"] for all. An array of strings.`},
		"rules":  {Type: "array", Desc: "Array of rule objects."},
		"id":     {Type: "string", Desc: "Pass several as a comma-separated string."},
		"filter": {JSONValue: true, Desc: "A JSON filter; arrays are allowed inside it."},
	})

	assert.Len(t, problems, 1)
	assert.Contains(t, problems[0], "topic is declared \"string\"")
}

func TestCrossToolActionMentions(t *testing.T) {
	actions := map[string][]string{
		"gateway_events_read":  {"list", "list_ignored"},
		"gateway_event_read":   {"get", "raw_body"},
		"gateway_request_read": {"get", "raw_body"},
	}

	t.Run("an action the named tool does not have", func(t *testing.T) {
		problems := CrossToolActionMentions(
			"Get one from gateway_events_read list, or from a request's events action on gateway_request_read.",
			actions)
		assert.Len(t, problems, 1)
		assert.Contains(t, problems[0], `the "events" action on gateway_request_read`)
	})

	t.Run("an action the named tool has", func(t *testing.T) {
		assert.Empty(t, CrossToolActionMentions("Get one from gateway_events_read list.", actions))
	})

	t.Run("an ordinary word after a tool name is not an action", func(t *testing.T) {
		assert.Empty(t, CrossToolActionMentions(
			"To act on one event by ID, use gateway_event_read instead.", actions))
	})

	t.Run("a tool this server does not advertise is left alone", func(t *testing.T) {
		assert.Empty(t, CrossToolActionMentions("see outpost_tenants_read list", actions))
	})
}

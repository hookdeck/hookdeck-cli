package mcpcore

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Prop describes a single JSON Schema property.
//
// Write marks a property only the write actions use, so it can be dropped in
// read-only mode alongside the actions themselves. Without it a read-only
// session was still offered `config`, `type` and `rules` — parameters belonging
// to actions it had not been given, with nothing in the schema to say so.
type Prop struct {
	Type  string   `json:"type"`
	Desc  string   `json:"description,omitempty"`
	Enum  []string `json:"enum,omitempty"`
	Items *Prop    `json:"items,omitempty"`
	Write bool     `json:"-"`

	// Actions restricts the property to the named actions. Empty means every
	// action on the tool it appears on.
	//
	// The group split put reads and writes on separate tools, which stops a
	// write-only property being offered to a read-only caller. It does not stop
	// a LIST filter being accepted on a by-id action of the same tool:
	// {action:"get", id:"web_1", disabled:true} was taken, `disabled` ignored,
	// and the caller handed one connection as though the filter had applied.
	// Same failure one level down, and the reason main carried a per-action
	// argument whitelist before v3.0.0.
	//
	// Declared on the property rather than in a parallel action->args map, so
	// there is one place to change when an action gains an argument.
	Actions []string `json:"-"`

	// Only restricts the property to the named action groups. Empty means every
	// group the resource renders.
	//
	// Filtering per group, not per mode, is the point: gateway_connections_pause
	// takes an id and nothing else, so advertising the twenty list filters there
	// shows a caller affordances that action cannot use — the same failure as
	// showing a write-only property to a read-only session, one level down.
	//
	// Write is the common case of this and stays as its own flag: it means
	// Only{GroupWrite}.
	Only []string `json:"-"`

	// JSONValue marks a property that accepts either a JSON object or a string
	// containing JSON — the Hookdeck filter syntax, which the payload filters
	// take in both forms.
	//
	// These were declared as plain strings while their descriptions said "object
	// or string" and their documented examples passed objects. A caller reading
	// the schema was told something the tool did not mean, and a strict client
	// would have rejected the documented usage.
	JSONValue bool `json:"-"`

	// ActionNotes are clauses appended to Desc, each rendered only on the tools
	// that carry at least one of the actions it names.
	//
	// A property shared by a resource's read and write tools had one
	// description, written when the resource was one tool, and it went on
	// naming every action after the split. outpost_tenants_write said its id
	// "On list, filters by tenant ID(s)" and gateway_connections_write said its
	// destination_id "Filters on list" — on tools with no list action;
	// outpost_events_read said its id was "Required for get/retry" with no
	// retry to be found. Prose describing a call the tool will refuse.
	//
	// Per-tool wording rather than deletion, because the clauses are true on
	// the sibling. Writing it as notes rather than as a description per group
	// keeps it derived: a note cannot name an action the tool does not have,
	// and an action moved between groups takes its clause with it.
	ActionNotes []ActionNote `json:"-"`
}

// ActionNote is one clause of a property description, scoped to the actions it
// is about. See Prop.ActionNotes.
type ActionNote struct {
	// On names the actions the clause describes.
	On []string

	// Text is the clause. A %s in it is filled with the actions from On that
	// the tool being rendered actually offers, joined with "/", so
	// "Required for %s." reads "Required for get." on the read tool and
	// "Required for upsert/delete/token/portal." on the write one.
	Text string
}

// namesIn returns the note's actions that this action set offers, in the order
// the note lists them.
func (n ActionNote) namesIn(actions ActionSet) []string {
	var names []string
	for _, name := range n.On {
		if _, ok := actions.Find(name); ok {
			names = append(names, name)
		}
	}
	return names
}

// renderFor resolves a property's ActionNotes against the actions one tool
// offers, leaving a plain description behind.
func (p Prop) renderFor(actions ActionSet) Prop {
	if len(p.ActionNotes) == 0 {
		return p
	}
	parts := make([]string, 0, len(p.ActionNotes)+1)
	if desc := strings.TrimSpace(p.Desc); desc != "" {
		parts = append(parts, desc)
	}
	for _, note := range p.ActionNotes {
		names := note.namesIn(actions)
		if len(names) == 0 {
			continue
		}
		text := note.Text
		if strings.Contains(text, "%s") {
			text = fmt.Sprintf(text, strings.Join(names, "/"))
		}
		if text = strings.TrimSpace(text); text != "" {
			parts = append(parts, text)
		}
	}
	p.Desc = strings.Join(parts, " ")
	p.ActionNotes = nil
	return p
}

// MarshalJSON emits the property as JSON Schema.
//
// It exists for JSONValue, which has to widen "type" to a list. Everything else
// marshals as the struct tags describe.
func (p Prop) MarshalJSON() ([]byte, error) {
	out := map[string]interface{}{}
	switch {
	case p.JSONValue:
		out["type"] = []string{"object", "string"}
	case p.Type != "":
		out["type"] = p.Type
	}
	if p.Desc != "" {
		out["description"] = p.Desc
	}
	if len(p.Enum) > 0 {
		out["enum"] = p.Enum
	}
	if p.Items != nil {
		out["items"] = p.Items
	}
	return json.Marshal(out)
}

// Schema builds a JSON Schema object with the given properties and required fields.
func Schema(properties map[string]Prop, required ...string) json.RawMessage {
	s := map[string]interface{}{
		"type":       "object",
		"properties": properties,
		// An unknown argument is always a caller error — a typo, or a filter the
		// caller assumed exists. Accepting it silently returned an unfiltered
		// result that read as a filtered one: gateway_events with request_id set
		// returned every event, because there is no such filter here.
		"additionalProperties": false,
	}
	if len(required) > 0 {
		s["required"] = required
	}
	data, _ := json.Marshal(s)
	return data
}

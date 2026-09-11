package mcpcore

import "encoding/json"

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

	// JSONValue marks a property that accepts either a JSON object or a string
	// containing JSON — the Hookdeck filter syntax, which the payload filters
	// take in both forms.
	//
	// These were declared as plain strings while their descriptions said "object
	// or string" and their documented examples passed objects. A caller reading
	// the schema was told something the tool did not mean, and a strict client
	// would have rejected the documented usage.
	JSONValue bool `json:"-"`
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

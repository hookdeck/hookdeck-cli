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
}

// Schema builds a JSON Schema object with the given properties and required fields.
func Schema(properties map[string]Prop, required ...string) json.RawMessage {
	s := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		s["required"] = required
	}
	data, _ := json.Marshal(s)
	return data
}

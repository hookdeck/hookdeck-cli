package mcpcore

import "encoding/json"

// Prop describes a single JSON Schema property.
type Prop struct {
	Type  string   `json:"type"`
	Desc  string   `json:"description,omitempty"`
	Enum  []string `json:"enum,omitempty"`
	Items *Prop    `json:"items,omitempty"`
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
